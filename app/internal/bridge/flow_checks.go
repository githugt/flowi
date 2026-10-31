// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/report"
	"github.com/githugt/flowi/lib/schemainfer"

	"github.com/githugt/flowi/app/internal/vm"
)

// The tools of an open flow that go beyond Run: a matrix of runs, the access check, the drift check and the sequence
// diagram of the last run. They run on the calling goroutine, never the actor, because they wait for runs.

// ChecksChoices are what the tool dialogs offer to pick from.
type ChecksChoices struct {
	Envs     []string `json:"envs"`     // environments of the workspace
	Overlays []string `json:"overlays"` // flowi overlays of the flow's collection: identities for the access check
	Inputs   []string `json:"inputs"`   // the flow's declared inputs
	Steps    []string `json:"steps"`    // its top-level request steps
}

// MatrixDimView is one axis of a matrix run: env, overlay or input.<name>.
type MatrixDimView struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

// MatrixCellView is one combination and how its run ended.
type MatrixCellView struct {
	Labels []string `json:"labels"`
	Status string   `json:"status"` // passed, failed, error or cancelled
	Why    string   `json:"why,omitempty"`
}

// MatrixView is the grid of a matrix run.
type MatrixView struct {
	Dims   []MatrixDimView  `json:"dims"`
	Cells  []MatrixCellView `json:"cells"`
	Passed int              `json:"passed"`
}

// AccessAttemptView is one step tried as one other identity.
type AccessAttemptView struct {
	Identity string `json:"identity"`
	Status   int    `json:"status"`
	Verdict  string `json:"verdict"` // denied, allowed or unclear
}

// AccessStepView is a step: its status for the owner, then what each other identity got.
type AccessStepView struct {
	ID       string              `json:"id"`
	Owner    int                 `json:"owner"`
	Attempts []AccessAttemptView `json:"attempts"`
}

// AccessView is the result of the access check.
type AccessView struct {
	Owner   string           `json:"owner"`
	Steps   []AccessStepView `json:"steps"`
	Exposed bool             `json:"exposed"` // another identity reached what the owner made
}

// DifferenceView is one place a response differs from its stored schema.
type DifferenceView struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"` // new, missing or changed
	Detail string `json:"detail,omitempty"`
	Fails  bool   `json:"fails"` // this kind fails the check
}

// RequestDriftView is one request's response against its schema.
type RequestDriftView struct {
	Request     string           `json:"request"`
	Version     string           `json:"version"`
	Status      int              `json:"status"`
	NoSchema    bool             `json:"noSchema"`
	Differences []DifferenceView `json:"differences"`
}

// DriftView is the result of the drift check.
type DriftView struct {
	RunStatus string             `json:"runStatus"`
	Requests  []RequestDriftView `json:"requests"`
	Failing   bool               `json:"failing"`
}

// SequenceStepView is one arrow or note of the diagram.
type SequenceStepView struct {
	Kind   string `json:"kind"` // call, reply or note
	Host   int    `json:"host"` // -1 for a note
	Text   string `json:"text"`
	Failed bool   `json:"failed"`
}

// SequenceView is the last run drawn as a sequence diagram, and as Mermaid text to copy.
type SequenceView struct {
	Title   string             `json:"title"`
	Hosts   []string           `json:"hosts"`
	Steps   []SequenceStepView `json:"steps"`
	Mermaid string             `json:"mermaid"`
}

// flowTarget is the open flow's target, once it is saved (a run uses the file on disk).
func (s *FlowService) flowTarget(node string) (flowi.FlowTarget, model.Flow, error) {
	var t flowi.FlowTarget
	var f model.Flow
	err := s.edit(node, func(o *openFlow) error {
		if o.doc.Dirty() {
			return errors.New("save the flow first: these use the file on disk")
		}
		t, f = o.doc.Target(), o.doc.Draft()
		return nil
	})
	return t, f, err
}

// CheckChoices lists what the dialogs offer: environments, the collection's overlays, the flow's inputs and steps.
func (s *FlowService) CheckChoices(node string) (ChecksChoices, error) {
	t, f, err := s.flowTarget(node)
	if err != nil {
		return ChecksChoices{}, err
	}
	v := ChecksChoices{Envs: []string{}, Overlays: []string{}, Inputs: []string{}, Steps: []string{}}
	for _, e := range s.app.backend.Environments() {
		v.Envs = append(v.Envs, e.Name)
	}
	if ovs, err := s.app.backend.Overlays(t.Collection); err == nil {
		for _, o := range ovs {
			v.Overlays = append(v.Overlays, o.Name)
		}
	}
	for _, in := range f.Inputs {
		v.Inputs = append(v.Inputs, in.Name)
	}
	for _, n := range f.Nodes {
		if n.Type == "request" {
			v.Steps = append(v.Steps, n.ID)
		}
	}
	return v, nil
}

// RunMatrix runs the saved flow once for every combination of the dimensions' values and returns the grid. inputs are
// values for the flow's inputs that stay the same in every cell.
func (s *FlowService) RunMatrix(node string, dims []MatrixDimView, inputs map[string]string) (MatrixView, error) {
	t, _, err := s.flowTarget(node)
	if err != nil {
		return MatrixView{}, err
	}
	vd := make([]vm.MatrixDim, len(dims))
	for i, d := range dims {
		vd[i] = vm.MatrixDim{Name: d.Name, Values: d.Values}
	}
	cells, err := vm.RunMatrix(context.Background(), s.app.backend, t, flowi.RunOptions{Inputs: inputs}, vd)
	if err != nil {
		return MatrixView{}, err
	}
	v := MatrixView{Dims: dims, Cells: []MatrixCellView{}}
	for _, c := range cells {
		v.Cells = append(v.Cells, MatrixCellView{Labels: c.Labels, Status: c.Status, Why: c.Why})
		if c.Status == "passed" {
			v.Passed++
		}
	}
	return v, nil
}

// CheckAccess runs the flow as the owner identity (an overlay), then tries the steps (or the ones that read what an
// earlier step made) as each other identity.
func (s *FlowService) CheckAccess(node, owner string, others, steps []string, inputs map[string]string) (AccessView, error) {
	if owner == "" || len(others) == 0 {
		return AccessView{}, errors.New("choose the owner identity and at least one other identity to try")
	}
	t, _, err := s.flowTarget(node)
	if err != nil {
		return AccessView{}, err
	}
	rep, err := s.app.backend.CheckAccess(context.Background(), t, flowi.AccessOptions{
		Env: s.app.workspace.SelectedEnv(), Owner: owner, Others: others, Steps: steps, Inputs: inputs,
	})
	if err != nil {
		return AccessView{}, err
	}
	v := AccessView{Owner: rep.Owner, Steps: []AccessStepView{}, Exposed: rep.Exposed()}
	for _, st := range rep.Steps {
		sv := AccessStepView{ID: st.ID, Owner: st.Owner, Attempts: []AccessAttemptView{}}
		for _, a := range st.Attempts {
			sv.Attempts = append(sv.Attempts, AccessAttemptView{Identity: a.Identity, Status: a.Status, Verdict: string(a.Verdict)})
		}
		v.Steps = append(v.Steps, sv)
	}
	return v, nil
}

// CheckDrift runs the flow without learning and compares each response with its stored schema. failOn lists the
// kinds that fail the check: new, missing, changed.
func (s *FlowService) CheckDrift(node, env string, failOn []string) (DriftView, error) {
	for _, k := range failOn {
		if !slices.Contains([]string{"new", "missing", "changed"}, k) {
			return DriftView{}, fmt.Errorf("fail on new, missing or changed, not %q", k)
		}
	}
	t, _, err := s.flowTarget(node)
	if err != nil {
		return DriftView{}, err
	}
	if env == "" {
		env = s.app.workspace.SelectedEnv()
	}
	rep, err := s.app.backend.CheckDrift(context.Background(), t, flowi.DriftOptions{Env: env})
	if err != nil {
		return DriftView{}, err
	}
	kinds := make([]schemainfer.DriftKind, len(failOn))
	for i, k := range failOn {
		kinds[i] = schemainfer.DriftKind(k)
	}
	v := DriftView{RunStatus: string(rep.RunStatus), Requests: []RequestDriftView{}, Failing: rep.Has(kinds...)}
	for _, q := range rep.Requests {
		qv := RequestDriftView{Request: q.Request, Version: q.Version, Status: q.Status, NoSchema: q.NoSchema, Differences: []DifferenceView{}}
		for _, d := range q.Differences {
			qv.Differences = append(qv.Differences, DifferenceView{Path: d.Path, Kind: string(d.Kind), Detail: d.Detail, Fails: slices.Contains(kinds, d.Kind)})
		}
		v.Requests = append(v.Requests, qv)
	}
	return v, nil
}

// Sequence draws the last run of the open flow as a sequence diagram: the hosts it called, each attempt with its
// status and time, and a note for a step that failed. Mermaid is the same picture as text.
func (s *FlowService) Sequence(node string) (SequenceView, error) {
	var run report.Run
	err := s.edit(node, func(o *openFlow) error {
		run = o.run.Result().Report
		return nil
	})
	if err != nil {
		return SequenceView{}, err
	}
	if run.ID == "" {
		return SequenceView{}, errors.New("run the flow first: the diagram is drawn from a run")
	}
	seq := report.SequenceOf(run)
	v := SequenceView{Title: seq.Title, Hosts: nonNil(seq.Hosts), Steps: []SequenceStepView{}}
	for _, st := range seq.Steps {
		v.Steps = append(v.Steps, SequenceStepView{Kind: st.Kind, Host: st.Host, Text: st.Text, Failed: st.Failed})
	}
	var sb strings.Builder
	if err := report.WriteMermaid(&sb, run); err != nil {
		return SequenceView{}, err
	}
	v.Mermaid = sb.String()
	return v, nil
}
