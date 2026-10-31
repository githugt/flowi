// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/githugt/flowi/internal/vm"
)

// RunView is how the last run of a flow went, as the canvas and the header show it.
type RunView struct {
	Running  bool              `json:"running"`
	Error    string            `json:"error,omitempty"`
	Summary  string            `json:"summary,omitempty"` // "passed · seed 7 · report file"
	Statuses map[string]string `json:"statuses"`          // by step id
	Results  map[string]string `json:"results"`           // by step id: "passed · 120ms"
}

// VarsView is what the flow does with variables and secrets.
type VarsView struct {
	Set     []string `json:"set"`
	Secrets []string `json:"secrets"`
	Unset   []string `json:"unset"`
	// Values are the variables as the last run ended, masked and shortened to one line each;
	// empty before a run.
	Values []VarValue `json:"values"`
}

// VarValue is one variable and what it held when the run ended.
type VarValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// varValueWidth bounds a value shown in the variables bar.
const varValueWidth = 120

func varValuesOf(vars map[string]any) []VarValue {
	out := []VarValue{}
	for _, name := range slices.Sorted(maps.Keys(vars)) {
		out = append(out, VarValue{Name: name, Value: oneLine(vars[name], varValueWidth)})
	}
	return out
}

// oneLine shows a value as text on one line, cut to width runes.
func oneLine(v any, width int) string {
	text, ok := v.(string)
	if !ok {
		data, err := json.Marshal(v)
		if err != nil {
			text = fmt.Sprint(v)
		} else {
			text = string(data)
		}
	}
	text = strings.Join(strings.Fields(text), " ")
	if r := []rune(text); len(r) > width {
		return string(r[:width-1]) + "…"
	}
	return text
}

// PartView is one block of a step's input or output.
type PartView struct {
	Heading   string   `json:"heading"`
	Kind      string   `json:"kind"` // lines, table, code
	Lines     []string `json:"lines,omitempty"`
	Rows      []Pair   `json:"rows,omitempty"`
	Code      string   `json:"code,omitempty"`
	Lang      string   `json:"lang,omitempty"`
	Collapsed bool     `json:"collapsed,omitempty"`
}

// TestView is one check of a step.
type TestView struct {
	Name   string `json:"name"`
	Detail string `json:"detail,omitempty"`
	Passed bool   `json:"passed"`
}

// NodeData is what the last run did at a step: the Data tab.
type NodeData struct {
	Step      string     `json:"step"`
	Status    string     `json:"status"`
	Instances int        `json:"instances"`
	Duration  string     `json:"duration"`
	Input     []PartView `json:"input"`
	Output    []PartView `json:"output"`
	Tests     []TestView `json:"tests"`
	Diags     []string   `json:"diags"`
	// Kind tells the dock which tabs to offer: "request" (Input, Output, Checks), "messages"
	// (Messages, Checks) or "step" (Output).
	Kind     string        `json:"kind"`
	Size     string        `json:"size,omitempty"` // how big the response (or the messages) were
	Messages []StepMessage `json:"messages"`
}

// StepMessage is one message of a message request in a step, with the way it went.
type StepMessage struct {
	Direction string `json:"direction"` // sent or received
	Text      string `json:"text"`
	Lang      string `json:"lang,omitempty"`
}

// HistoryRow is a saved run in the Runs tab.
type HistoryRow struct {
	Label  string `json:"label"`
	Status string `json:"status"`
	ID     string `json:"id"`
}

// DiffView is one step in the comparison of a saved run with the newest.
type DiffView struct {
	Step    string `json:"step"`
	Name    string `json:"name"`
	Before  string `json:"before"`
	After   string `json:"after"`
	Gap     string `json:"gap"`
	Changed bool   `json:"changed"`
}

// FlowRunEvent is published as the "flowrun" event while a flow runs.
type FlowRunEvent struct {
	Node string  `json:"node"`
	Run  RunView `json:"run"`
}

// HistoryEvent is published as the "flowhistory" event when the saved runs load.
type HistoryEvent struct {
	Node string       `json:"node"`
	Runs []HistoryRow `json:"runs"`
}

// Run runs the saved flow in the selected environment. inputs override the
// flow's defaults.
func (s *FlowService) Run(node string, inputs map[string]string) {
	_ = s.edit(node, func(o *openFlow) error {
		o.run.Run(s.app.workspace.SelectedEnv(), inputs)
		return nil
	})
}

// RunPart runs part of the flow around the selected step: scope is only,
// until or from.
func (s *FlowService) RunPart(node, scope string, inputs map[string]string) error {
	return s.edit(node, func(o *openFlow) error {
		sc, ok := map[string]vm.RunScope{"only": vm.ScopeOnly, "until": vm.ScopeUntil, "from": vm.ScopeFrom}[scope]
		if !ok {
			return fmt.Errorf("unknown scope %q", scope)
		}
		if !o.run.RunPart(s.app.workspace.SelectedEnv(), inputs, o.selected, sc) {
			return fmt.Errorf("select a step to run")
		}
		return nil
	})
}

// Cancel stops a run in progress.
func (s *FlowService) Cancel(node string) {
	_ = s.edit(node, func(o *openFlow) error { o.run.Cancel(); return nil })
}

// ClearResults forgets the last run's statuses and data.
func (s *FlowService) ClearResults(node string) {
	_ = s.edit(node, func(o *openFlow) error {
		if !o.run.Running() {
			o.run.ClearResults()
		}
		return nil
	})
}

// NodeData is what the last run did at a step; format is json, text or raw.
func (s *FlowService) NodeData(node, step, format string) (NodeData, error) {
	var data NodeData
	err := s.edit(node, func(o *openFlow) error {
		n, ok := o.doc.Node(step)
		if !ok {
			return fmt.Errorf("no step %q", step)
		}
		d, ran := o.run.Detail(step)
		if !ran {
			data = NodeData{Step: step, Input: []PartView{}, Output: []PartView{}, Tests: []TestView{}, Diags: []string{}, Messages: []StepMessage{}}
			return nil
		}
		f := map[string]vm.ViewFormat{"json": vm.ViewJSON, "text": vm.ViewText, "raw": vm.ViewRaw}[format]
		data = NodeData{
			Step: step, Status: d.Status, Instances: d.Instances, Duration: d.Duration.Round(time.Millisecond).String(),
			Input: partsOf(vm.InputParts(n, d)), Output: partsOf(vm.OutputParts(n, d, f)),
			Tests: testsOf(vm.TestRows(n, d)), Diags: append([]string{}, d.Diags...),
			Kind: dataKind(d), Size: vm.RecordSize(d), Messages: stepMessagesOf(vm.MessageRows(d)),
		}
		return nil
	})
	return data, err
}

// LoadHistory reads the saved runs; the "flowhistory" event delivers them.
func (s *FlowService) LoadHistory(node string) {
	_ = s.edit(node, func(o *openFlow) error { o.history.Load(); return nil })
}

// CompareRuns compares saved run from with saved run to (indexes, newest
// first): the status and time of each top-level step.
func (s *FlowService) CompareRuns(node string, from, to int) []DiffView {
	out := []DiffView{}
	_ = s.edit(node, func(o *openFlow) error {
		for _, r := range o.history.Between(from, to) {
			out = append(out, DiffView{Step: r.Node, Name: r.Name, Before: r.Before, After: r.After, Gap: r.DurationGap, Changed: r.Changed})
		}
		return nil
	})
	return out
}

// BodyLineView is one line of a step's outputs in a comparison.
type BodyLineView struct {
	Kind string `json:"kind"` // same, add (only in the later run) or del (only in the earlier)
	Text string `json:"text"`
}

// VarChangeView is a variable that ended differently in two runs.
type VarChangeView struct {
	Name   string `json:"name"`
	Before string `json:"before"`
	After  string `json:"after"`
	// Had says which runs had the variable at all.
	HadBefore bool `json:"hadBefore"`
	HadAfter  bool `json:"hadAfter"`
}

// RunComparisonView is what two runs output at each step, line by line, and
// which variables ended differently.
type RunComparisonView struct {
	Steps map[string][]BodyLineView `json:"steps"` // only steps whose outputs differ
	Vars  []VarChangeView           `json:"vars"`
}

// CompareRunDetail reads two saved runs and compares what each step output
// and the variables the flow ended with. The earlier run is from.
func (s *FlowService) CompareRunDetail(node string, from, to int) (RunComparisonView, error) {
	type result struct {
		v   RunComparisonView
		err error
	}
	done := make(chan result, 1)
	if err := s.edit(node, func(o *openFlow) error {
		o.history.CompareInDepth(from, to, func(c vm.RunComparison, err error) { done <- result{runComparisonViewOf(c), err} })
		return nil
	}); err != nil {
		return RunComparisonView{}, err
	}
	r := <-done
	return r.v, r.err
}

func runComparisonViewOf(c vm.RunComparison) RunComparisonView {
	out := RunComparisonView{Steps: map[string][]BodyLineView{}, Vars: []VarChangeView{}}
	for id, lines := range c.Nodes {
		for _, l := range lines {
			out.Steps[id] = append(out.Steps[id], BodyLineView{Kind: l.Kind, Text: l.Text})
		}
	}
	for _, v := range c.Vars {
		out.Vars = append(out.Vars, VarChangeView{Name: v.Name, Before: v.Before, After: v.After, HadBefore: v.HadBefore, HadAfter: v.HadAfter})
	}
	return out
}

// runViewOf builds the RunView; call it on the actor.
func runViewOf(o *openFlow) RunView {
	v := RunView{Running: o.run.Running(), Error: o.run.Err(), Statuses: map[string]string{}, Results: map[string]string{}}
	for _, id := range o.doc.NodeIDs() {
		d, ok := o.run.Detail(id)
		if !ok || d.Status == "" {
			continue
		}
		v.Statuses[id] = d.Status
		v.Results[id] = d.Status
		if d.Duration > 0 {
			v.Results[id] += " · " + d.Duration.Round(time.Millisecond).String()
		}
	}
	if r := o.run.Result(); r.RunID != "" && v.Error == "" && !v.Running {
		v.Summary = fmt.Sprintf("%s · seed %d", r.Status, r.Seed)
	}
	return v
}

func varsOf(o *openFlow) VarsView {
	f := vm.ScanFlow(o.doc.Draft())
	return VarsView{Set: nonNil(f.Set), Secrets: nonNil(f.Secrets), Unset: nonNil(f.Unset), Values: varValuesOf(o.run.Vars())}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func partsOf(parts []vm.Part) []PartView {
	out := make([]PartView, 0, len(parts))
	for _, p := range parts {
		pv := PartView{Heading: p.Heading, Lines: p.Lines, Rows: pairsOf(p.Rows), Code: p.Code, Lang: p.Lang, Collapsed: p.Collapsed}
		switch p.Kind {
		case vm.PartTable:
			pv.Kind = "table"
		case vm.PartCode:
			pv.Kind = "code"
		default:
			pv.Kind = "lines"
		}
		out = append(out, pv)
	}
	return out
}

func testsOf(rows []vm.TestRow) []TestView {
	out := make([]TestView, 0, len(rows))
	for _, r := range rows {
		out = append(out, TestView{Name: r.Name, Detail: r.Detail, Passed: r.Passed})
	}
	return out
}

func historyOf(h *vm.RunHistory) []HistoryRow {
	out := []HistoryRow{}
	for _, r := range h.Runs() {
		out = append(out, HistoryRow{Label: vm.RunLabel(r), Status: r.Status, ID: r.ID})
	}
	return out
}

// dataKind says which tabs the output dock offers for a step.
func dataKind(d vm.NodeDetail) string {
	switch {
	case d.Record != nil && d.Record.HasMessages():
		return "messages"
	case d.Record != nil:
		return "request"
	}
	return "step"
}

func stepMessagesOf(rows []vm.MessageRow) []StepMessage {
	out := make([]StepMessage, 0, len(rows))
	for _, r := range rows {
		out = append(out, StepMessage{Direction: r.Direction, Text: r.Text, Lang: r.Lang})
	}
	return out
}
