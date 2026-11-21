// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

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
	Detail string   `json:"detail,omitempty"` // shown beside the mark of a passed cell: a request's response status
}

// MatrixView is the grid of a matrix run.
type MatrixView struct {
	Dims   []MatrixDimView  `json:"dims"`
	Cells  []MatrixCellView `json:"cells"`
	Passed int              `json:"passed"`
	// Total is how many cells the matrix has, Cancelled that it was stopped before the last: Cells then holds only
	// the ones that ran.
	Total     int  `json:"total"`
	Cancelled bool `json:"cancelled"`
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
	// Checked is when the check ran, in local time ("2006-01-02 15:04"); "" when there is no check to show.
	Checked string `json:"checked"`
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
	Accepted    bool             `json:"accepted"` // its schema was brought up to date from this check
}

// DriftView is the result of the drift check.
type DriftView struct {
	RunStatus string             `json:"runStatus"`
	Requests  []RequestDriftView `json:"requests"`
	Failing   bool               `json:"failing"`
	// CI is the command that runs this check from a script or CI, to copy.
	CI string `json:"ci"`
	// Note says what accepting did when that is worth saying (a schema kept from a spec is never rewritten).
	Note string `json:"note,omitempty"`
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
// values for the flow's inputs that stay the same in every cell. Progress arrives as the "matrix" event, and
// CancelMatrix stops it with the cells that ran so far returned (Cancelled set).
func (s *FlowService) RunMatrix(node string, dims []MatrixDimView, inputs map[string]string) (MatrixView, error) {
	t, _, err := s.flowTarget(node)
	if err != nil {
		return MatrixView{}, err
	}
	ctx, finish, err := s.app.matrices.begin(node)
	if err != nil {
		return MatrixView{}, err
	}
	defer finish()
	cells, err := vm.RunMatrixProgress(ctx, s.app.backend, t, flowi.RunOptions{Inputs: inputs}, matrixDims(dims), s.app.matrices.progress(s.app.emit, node))
	return matrixResult(dims, cells, err)
}

// CancelMatrix stops the matrix running for the node, if any; the grid so far comes back from RunMatrix.
func (s *FlowService) CancelMatrix(node string) { s.app.matrices.cancel(node) }

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
	return accessViewOf(rep, time.Now()), nil
}

// LastAccess is the access check last run for the flow, which is kept so the table is still there after the dialog
// closes or the app restarts; an empty Owner says there is none.
//
// Before: the results were lost when the dialog closed.
func (s *FlowService) LastAccess(node string) (AccessView, error) {
	t, _, err := s.flowTarget(node)
	if err != nil {
		return AccessView{}, err
	}
	saved, ok := s.app.backend.LastAccess(t)
	if !ok {
		return AccessView{Steps: []AccessStepView{}}, nil
	}
	return accessViewOf(saved.Report, saved.At), nil
}

// accessViewOf is a check as the dialog shows it, with the time it ran.
func accessViewOf(rep flowi.AccessReport, at time.Time) AccessView {
	v := AccessView{Owner: rep.Owner, Steps: []AccessStepView{}, Exposed: rep.Exposed(), Checked: at.Local().Format("2006-01-02 15:04")}
	for _, st := range rep.Steps {
		sv := AccessStepView{ID: st.ID, Owner: st.Owner, Attempts: []AccessAttemptView{}}
		for _, a := range st.Attempts {
			sv.Attempts = append(sv.Attempts, AccessAttemptView{Identity: a.Identity, Status: a.Status, Verdict: string(a.Verdict)})
		}
		v.Steps = append(v.Steps, sv)
	}
	return v
}

// CheckDrift runs the flow without learning and compares each response with its stored schema. failOn lists the
// kinds that fail the check: new, missing, changed.
func (s *FlowService) CheckDrift(node, env string, failOn []string) (DriftView, error) {
	for _, k := range failOn {
		if !slices.Contains([]string{"new", "missing", "changed"}, k) {
			return DriftView{}, fmt.Errorf("fail on new, missing or changed, not %q", k)
		}
	}
	t, f, err := s.flowTarget(node)
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
	var v DriftView
	_ = s.edit(node, func(o *openFlow) error {
		o.drift, o.driftFail, o.driftEnv, o.accepted = &rep, kinds, env, map[string]bool{}
		v = s.driftView(o, t, f)
		return nil
	})
	return v, nil
}

// driftView is the last drift check as the dialog shows it: the requests accepted since have nothing left to show.
func (s *FlowService) driftView(o *openFlow, t flowi.FlowTarget, f model.Flow) DriftView {
	rep := o.drift
	v := DriftView{RunStatus: string(rep.RunStatus), Requests: []RequestDriftView{}, CI: s.driftCommand(t, f, o.driftEnv, o.driftFail)}
	for _, q := range rep.Requests {
		qv := RequestDriftView{Request: q.Request, Version: q.Version, Status: q.Status, NoSchema: q.NoSchema && !o.accepted[q.Request], Differences: []DifferenceView{}, Accepted: o.accepted[q.Request]}
		if !qv.Accepted {
			for _, d := range q.Differences {
				fails := slices.Contains(o.driftFail, d.Kind)
				qv.Differences = append(qv.Differences, DifferenceView{Path: d.Path, Kind: string(d.Kind), Detail: d.Detail, Fails: fails})
				v.Failing = v.Failing || fails
			}
		}
		v.Requests = append(v.Requests, qv)
	}
	return v
}

// driftCommand is the command line that runs the same check from a script or CI.
func (s *FlowService) driftCommand(t flowi.FlowTarget, f model.Flow, env string, kinds []schemainfer.DriftKind) string {
	parts := []string{"flowi", "drift", shellWord(f.Path)}
	for _, c := range s.app.workspace.Collections() {
		if c.Manifest.ID == t.Collection {
			parts = append(parts, "--collection", shellWord(c.Manifest.Name))
		}
	}
	if env != "" {
		parts = append(parts, "--env", shellWord(env))
	}
	names := make([]string, len(kinds))
	for i, k := range kinds {
		names[i] = string(k)
	}
	if len(names) > 0 {
		parts = append(parts, "--fail-on", strings.Join(names, ","))
	}
	return strings.Join(parts, " ")
}

// shellWord quotes a word for a shell when it needs it.
func shellWord(w string) string {
	if w != "" && strings.Trim(w, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._/-=:,") == "" {
		return w
	}
	return "'" + strings.ReplaceAll(w, "'", `'\''`) + "'"
}

// AcceptDrift brings the stored schemas up to date with what the last drift check saw, for the requests listed (none:
// all of them), and returns the check with those requests settled. It does not run the flow again: it accepts what the
// person was shown.
func (s *FlowService) AcceptDrift(node string, requests []string) (DriftView, error) {
	var rep flowi.DriftReport
	var listed []string
	err := s.edit(node, func(o *openFlow) error {
		if o.drift == nil {
			return errors.New("run the check first: accepting is of what it found")
		}
		rep = *o.drift
		for _, q := range rep.Requests {
			if len(requests) == 0 || slices.Contains(requests, q.Request) {
				listed = append(listed, q.Request)
			}
		}
		return nil
	})
	if err != nil {
		return DriftView{}, err
	}
	changes, err := s.app.backend.AcceptDrift(rep, listed)
	if err != nil {
		return DriftView{}, err
	}
	t, f, terr := s.flowTarget(node)
	if terr != nil {
		return DriftView{}, terr
	}
	var v DriftView
	_ = s.edit(node, func(o *openFlow) error {
		for _, r := range listed {
			o.accepted[r] = true
		}
		v = s.driftView(o, t, f)
		if len(changes) == 0 {
			v.Note = "No schema changed: a schema that came from a spec or was edited by hand is never rewritten."
		}
		return nil
	})
	return v, nil
}

// Sequence draws the last run of the open flow as a sequence diagram: the hosts it called, each attempt with its
// status and time, and a note for a step that failed. others also notes the steps that call nothing (delays, sets,
// assertions). Mermaid is the same picture as text.
func (s *FlowService) Sequence(node string, others bool) (SequenceView, error) {
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
	seq := report.SequenceWith(run, others)
	v := SequenceView{Title: seq.Title, Hosts: nonNil(seq.Hosts), Steps: []SequenceStepView{}}
	for _, st := range seq.Steps {
		v.Steps = append(v.Steps, SequenceStepView{Kind: st.Kind, Host: st.Host, Text: st.Text, Failed: st.Failed})
	}
	var sb strings.Builder
	if err := report.WriteSequence(&sb, seq); err != nil {
		return SequenceView{}, err
	}
	v.Mermaid = sb.String()
	return v, nil
}
