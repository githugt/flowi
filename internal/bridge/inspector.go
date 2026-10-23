// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"
	"slices"
	"strconv"
	"time"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/analysis"
	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/canvas"
	"github.com/githugt/flowi/internal/vm"
)

// Pair is one row of a key/value editor.
type Pair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Choice is a request or flow a step can call.
type Choice struct {
	ID    string `json:"id"` // what is stored; together with Path it names the reference
	Path  string `json:"path"`
	Label string `json:"label"`
	Dep   string `json:"dep,omitempty"` // the dependency collection it is in
}

// Param is one parameter row of a step.
type Param struct {
	Name     string   `json:"name"`
	Kind     string   `json:"kind"` // string, text, expr, duration, int, bool, enum, name, any, map
	Doc      string   `json:"doc"`
	Required bool     `json:"required"`
	Enum     []string `json:"enum,omitempty"`
	Value    string   `json:"value"`
	Complex  bool     `json:"complex,omitempty"` // a list or mapping: edited in the file
}

// Call is what a request or subflow step calls.
type Call struct {
	Kind    string   `json:"kind"` // request or subflow
	Ref     string   `json:"ref"`
	Choices []Choice `json:"choices"`
	Version string   `json:"version,omitempty"`
	With    []Pair   `json:"with"`
	Headers []Pair   `json:"headers,omitempty"`
	Hooks   string   `json:"hooks,omitempty"`
}

// Lane is a case of a condition or a branch of a parallel.
type Lane struct {
	Label string `json:"label"`
	When  string `json:"when,omitempty"`
	Empty bool   `json:"empty"` // only an empty lane can be removed
}

// StepForm is the inspector of one step.
type StepForm struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Group       string   `json:"group"`
	Summary     string   `json:"summary"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Needs       []string `json:"needs"`       // the steps it waits for besides the one before it
	Previous    string   `json:"previous"`    // the step before it, which it follows without a needs
	NeedChoices []string `json:"needChoices"` // the steps before it it could also wait for
	Uses        []Use    `json:"uses"`        // values it reads from other steps
	UsedBy      []Use    `json:"usedBy"`      // steps that read values it makes
	When        string   `json:"when"`
	Timeout     string   `json:"timeout"`
	Concurrent  bool     `json:"concurrent"`
	FailMode    string   `json:"failMode"` // fail, continue, goto
	FailTarget  string   `json:"failTarget"`
	Steps       []string `json:"steps"`    // the other steps, for a goto
	Attach      string   `json:"attach"`   // the request or subflow step this one is drawn with
	AttachAt    string   `json:"attachAt"` // where around it: above, below, or "" beside it
	AttachTo    []string `json:"attachTo"` // the steps it could be drawn with
	Attached    []string `json:"attached"` // for a request or subflow step: the steps drawn in one block with it
	Call        *Call    `json:"call,omitempty"`
	Params      []Param  `json:"params"`
	Variables   *[]Pair  `json:"variables,omitempty"`
	VarsHeading string   `json:"varsHeading,omitempty"` // "variables" or "extract"
	Intro       string   `json:"intro,omitempty"`       // "extract" for the read-one-value form
	Cases       []Lane   `json:"cases,omitempty"`
	Branches    []Lane   `json:"branches,omitempty"`
}

// Use is one value that passes from one step to another: From made it, To
// reads it. Name is the output or variable ("login.body", "token").
type Use struct {
	From string `json:"from"`
	To   string `json:"to"`
	Name string `json:"name"`
}

// FlowForm is the inspector of the flow itself, shown when no step is selected.
type FlowForm struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	PreferVersion  string `json:"preferVersion"`
	Inputs         []Pair `json:"inputs"`
	Outputs        []Pair `json:"outputs"`
	MaxConcurrency string `json:"maxConcurrency"`
	RateLimit      string `json:"rateLimit"`
	OnBranchError  string `json:"onBranchError"`
}

// Form is what the inspector shows: the selected step, or the flow.
type Form struct {
	Step *StepForm `json:"step,omitempty"`
	Flow FlowForm  `json:"flow"`
}

// PaletteEntry is a step type that can be added.
type PaletteEntry struct {
	Type    string `json:"type"`
	Group   string `json:"group"`
	Summary string `json:"summary"`
}

// catalog indexes the step types. It is built per call: the list is small and there is no global state.
func catalog() specIndex { return indexCatalog(flowi.NodeCatalog()) }

type specIndex struct {
	list   []flowi.NodeSpec
	byType map[string]flowi.NodeSpec
}

func indexCatalog(specs []flowi.NodeSpec) specIndex {
	idx := specIndex{list: specs, byType: map[string]flowi.NodeSpec{}}
	for _, s := range specs {
		idx.byType[s.Type] = s
	}
	return idx
}

// paletteOf lists every step type in catalogue order.
func paletteOf() []PaletteEntry {
	out := make([]PaletteEntry, 0, len(catalog().list))
	for _, s := range catalog().list {
		out = append(out, PaletteEntry{Type: s.Type, Group: s.Group, Summary: s.Summary})
	}
	return out
}

// formOf builds the inspector for target ("" for the flow); call it on the actor.
func (s *FlowService) formOf(d *vm.FlowDoc, target string) Form {
	f := Form{Flow: flowFormOf(d.Draft())}
	if n, ok := d.Node(target); ok && target != "" {
		step := s.stepFormOf(d, n)
		f.Step = &step
	}
	return f
}

func flowFormOf(f model.Flow) FlowForm {
	inputs := make([]Pair, 0, len(f.Inputs))
	for _, in := range f.Inputs {
		inputs = append(inputs, Pair{Key: in.Name, Value: in.Default})
	}
	return FlowForm{
		Name: f.Name, Description: f.Description, PreferVersion: f.PreferVersion,
		Inputs: inputs, Outputs: pairsOf(f.Outputs),
		MaxConcurrency: intText(f.Execution.MaxConcurrency), RateLimit: f.Execution.RateLimit, OnBranchError: f.Execution.OnBranchError,
	}
}

func (s *FlowService) stepFormOf(d *vm.FlowDoc, n model.Node) StepForm {
	spec := catalog().byType[string(n.Type)]
	mode, target := vm.ParseFailRoute(n.OnError)
	others := slices.DeleteFunc(d.NodeIDs(), func(id string) bool { return id == n.ID })
	st := StepForm{
		ID: n.ID, Type: string(n.Type), Group: spec.Group, Summary: spec.Summary,
		Name: n.Name, Description: n.Description, Needs: nonNil(n.Needs), Previous: d.Previous(n.ID), NeedChoices: nonNil(d.EarlierIDs(n.ID)), When: n.When,
		Timeout: durationText(n.Timeout), Concurrent: n.Concurrent,
		FailMode: string(mode), FailTarget: target, Steps: others, Attach: n.Attach, AttachAt: n.AttachAt, AttachTo: attachChoices(d, n), Attached: attachedTo(d, n),
		Call: s.callOf(d, n), Params: paramsOf(n, spec),
	}
	st.Uses, st.UsedBy = usesOf(d, n.ID)
	if n.Type == "extract" {
		st.Intro = "extract"
	}
	if spec.MapForm && vm.VariablesShown(n) {
		rows := variableRows(n, spec)
		st.Variables = &rows
		st.VarsHeading = "variables"
		if n.Type == "extract" {
			st.VarsHeading = "extract"
		}
	}
	for i, c := range n.Cases {
		st.Cases = append(st.Cases, Lane{Label: caseLabel(i), When: c.When, Empty: len(c.Nodes) == 0})
	}
	for _, b := range n.Branches {
		st.Branches = append(st.Branches, Lane{Label: b.ID, Empty: len(b.Nodes) == 0})
	}
	return st
}

func paramsOf(n model.Node, spec flowi.NodeSpec) []Param {
	out := []Param{}
	for _, p := range spec.Params {
		if !vm.ParamShown(n, p.Name) {
			continue
		}
		v := vm.ParamValue(n, p.Name)
		_, text := v.(string)
		param := Param{Name: p.Name, Kind: p.Kind, Doc: p.Doc, Required: p.Required, Enum: p.Enum, Value: vm.ParamText(n, p.Name)}
		switch v.(type) {
		case nil, bool, int, int64, float64, string:
		default:
			param.Complex = true // a list or mapping
		}
		if (p.Kind == "map" || p.Kind == "any") && v != nil && !text {
			param.Complex = true
		}
		out = append(out, param)
	}
	return out
}

func variableRows(n model.Node, spec flowi.NodeSpec) []Pair {
	rows := []Pair{}
	for _, f := range n.Params {
		if _, isSpec := spec.Param(f.Key); isSpec {
			continue
		}
		rows = append(rows, Pair{Key: f.Key, Value: fmt.Sprint(f.Value)})
	}
	return rows
}

func (s *FlowService) callOf(d *vm.FlowDoc, n model.Node) *Call {
	switch {
	case n.Request != nil:
		return &Call{
			Kind: "request", Ref: string(n.Request.Request.ID), Choices: s.requestChoices(d.Collection()),
			Version: n.Request.Version, With: pairsOf(n.Request.With), Headers: pairsOf(n.Request.Headers), Hooks: hookMode(n.Request.Hooks),
		}
	case n.Subflow != nil:
		return &Call{Kind: "subflow", Ref: string(n.Subflow.Flow.ID), Choices: flowChoices(d.Collection()), With: pairsOf(n.Subflow.With)}
	}
	return nil
}

// requestChoices are the requests a flow in c can call: its own, then those
// of the collections it depends on (by dependency name).
func (s *FlowService) requestChoices(c model.Collection) []Choice {
	var out []Choice
	add := func(coll model.Collection, prefix, dep string) {
		for _, r := range coll.Requests {
			out = append(out, Choice{ID: string(r.ID), Path: r.Path, Label: prefix + r.Path, Dep: dep})
		}
	}
	add(c, "", "")
	for _, dep := range c.Manifest.DependsOn {
		if dc, ok := s.app.workspace.Collection(dep.ID); ok {
			add(dc, dep.Name+": ", dep.Name)
		}
	}
	return out
}

func flowChoices(c model.Collection) []Choice {
	var out []Choice
	for _, f := range c.Flows {
		out = append(out, Choice{ID: string(f.ID), Path: f.Path, Label: f.Path})
	}
	return out
}

func pairsOf(kv []model.KeyValue) []Pair {
	out := make([]Pair, 0, len(kv))
	for _, x := range kv {
		out = append(out, Pair{Key: x.Key, Value: x.Value})
	}
	return out
}

func keyValuesOf(rows []Pair) []model.KeyValue {
	out := make([]model.KeyValue, 0, len(rows))
	for _, r := range rows {
		out = append(out, model.KeyValue{Key: r.Key, Value: r.Value})
	}
	return out
}

func hookMode(h model.HookControl) string {
	switch {
	case h.Mode == "none":
		return vm.HooksNone
	case slices.Contains(h.Skip, "pre"):
		return vm.HooksSkipPre
	case slices.Contains(h.Skip, "post"):
		return vm.HooksSkipPost
	}
	return vm.HooksAll
}

func intText(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}

func durationText(d time.Duration) string {
	if d == 0 {
		return ""
	}
	return d.String()
}

// attachChoices are the steps n can be drawn with; a request or subflow step is
// the one others are drawn with, and a container is drawn as branches, so those have none.
func attachChoices(d *vm.FlowDoc, n model.Node) []string {
	switch n.Type {
	case "request", "subflow", "parallel", "condition", "batch", "run_till":
		return []string{} // these are what others are drawn with, or boxes of their own
	}
	choices := d.AttachChoices(n.ID)
	if choices == nil {
		return []string{}
	}
	return choices
}

// attachedTo are the steps drawn in one block with request or subflow step n.
func attachedTo(d *vm.FlowDoc, n model.Node) []string {
	out := []string{}
	if u, ok := d.Diagram().UnitOf(n.ID); ok && u.Kind == canvas.KindRequest {
		for i, id := range u.Nodes {
			if i != u.Core {
				out = append(out, id)
			}
		}
	}
	return out
}

// usesOf splits the data links of the flow into what node id reads and what
// reads node id.
func usesOf(d *vm.FlowDoc, id string) (reads, readBy []Use) {
	reads, readBy = []Use{}, []Use{}
	for _, l := range analysis.DataLinks(d.Draft()) {
		use := Use{From: l.From, To: l.To, Name: l.Via}
		if l.To == id {
			reads = append(reads, use)
		}
		if l.From == id {
			readBy = append(readBy, use)
		}
	}
	return reads, readBy
}

// caseLabel names case i of a condition: the first is the "if", the others "else if".
func caseLabel(i int) string {
	if i == 0 {
		return "If"
	}
	return "Else if"
}
