// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/githugt/flowi/app/internal/vm"
	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"
)

// RunView is how the last run of a flow went, as the canvas and the header show it.
type RunView struct {
	Running bool   `json:"running"`
	Error   string `json:"error,omitempty"`
	Summary string `json:"summary,omitempty"` // "passed · seed 7 · report file"
	// Seed is the last run's seed as text, so a copied `flowi run` command can repeat its generated values.
	Seed     string            `json:"seed,omitempty"`
	Statuses map[string]string `json:"statuses"` // by step id
	Results  map[string]string `json:"results"`  // by step id: "passed · 120ms"
	Timeline []TimelineBar     `json:"timeline"` // the steps on the run's clock, by start
}

// VarsView is what the flow does with variables and secrets.
type VarsView struct {
	Set     []string `json:"set"`
	Secrets []string `json:"secrets"`
	Unset   []string `json:"unset"`
	// Values are the variables as the last run ended, masked and shortened to one line each;
	// empty before a run.
	Values []VarValue `json:"values"`
	// Uses says which steps write and read each variable, by name.
	Uses []VarUse `json:"uses"`
}

// VarUse is the steps that write and read one variable, by step id.
type VarUse struct {
	Name    string   `json:"name"`
	Writers []string `json:"writers"`
	Readers []string `json:"readers"`
}

// varUsesOf lists the steps that write and read each variable, by name.
func varUsesOf(f vm.FlowVars) []VarUse {
	names := map[string]bool{}
	for n := range f.Writers {
		names[n] = true
	}
	for n := range f.Readers {
		names[n] = true
	}
	out := []VarUse{}
	for _, n := range slices.Sorted(maps.Keys(names)) {
		out = append(out, VarUse{Name: n, Writers: nonNil(f.Writers[n]), Readers: nonNil(f.Readers[n])})
	}
	return out
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
	// Table holds the rows of a dataset step, so the Output tab shows a table as the request's response pane does.
	Table *TableView `json:"table,omitempty"`
}

// StepMessage is one message of a message request in a step, with the way it went.
type StepMessage struct {
	Direction string `json:"direction"` // sent or received
	Text      string `json:"text"`
	Lang      string `json:"lang,omitempty"`
	// The fields below are what the message feed shows, the same as a request's own messages.
	At   string `json:"at"`
	Body string `json:"body"`
	Key  string `json:"key,omitempty"`
	Kind string `json:"kind,omitempty"`
	Type string `json:"type,omitempty"`
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
	// Trend is the latest runs as one line ("✓✓✗✓ · flaky · median 1.2 s"), and SlowNote names a step that ran much
	// longer than usual in the latest run; both are "" when there is nothing to say.
	Trend    string `json:"trend"`
	SlowNote string `json:"slowNote"`
}

// Run runs the saved flow in the selected environment. inputs override the
// flow's defaults.
func (s *FlowService) Run(node string, inputs map[string]string) {
	_ = s.edit(node, func(o *openFlow) error {
		o.run.Run(s.app.workspace.SelectedEnv(), inputs)
		return nil
	})
}

// Preflight says what a run now would need that the selected environment does not have: one plain line each, none
// when the run is ready or no environment is selected. It reads the files, so it runs off the actor.
func (s *FlowService) Preflight(node string) ([]string, error) {
	var target flowi.FlowTarget
	var draft model.Flow
	if err := s.edit(node, func(o *openFlow) error { target, draft = o.doc.Target(), o.doc.Draft(); return nil }); err != nil {
		return nil, err
	}
	env := s.app.workspace.SelectedEnv()
	needs, err := s.app.backend.PreflightFlow(target, env, &draft)
	if err != nil {
		return nil, err
	}
	return append([]string{}, vm.PreflightLines(env, needs)...), nil
}

// Writes says which steps of the flow change data, one plain line each, none when it only reads. The window asks
// before a run in a protected environment, and shows these lines in the question.
func (s *FlowService) Writes(node string) ([]string, error) {
	var target flowi.FlowTarget
	var draft model.Flow
	if err := s.edit(node, func(o *openFlow) error { target, draft = o.doc.Target(), o.doc.Draft(); return nil }); err != nil {
		return nil, err
	}
	writes, err := s.app.backend.FlowWrites(target, &draft)
	if err != nil {
		return nil, err
	}
	return append([]string{}, vm.WriteLines(writes)...), nil
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
			Kind: dataKind(d), Size: vm.RecordSize(d), Messages: stepMessagesOf(vm.MessageRows(d), d), Table: stepTable(d),
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
	v.Timeline = timelineOf(o.doc.NodeIDs(), o.run.Detail, o.run.Began(), time.Now())
	for _, id := range o.doc.NodeIDs() {
		d, ok := o.run.Detail(id)
		if !ok || d.Status == "" {
			continue
		}
		v.Statuses[id] = d.Status
		v.Results[id] = resultLine(d)
	}
	if r := o.run.Result(); r.RunID != "" && v.Error == "" && !v.Running {
		v.Summary = runSummary(string(r.Status), r.Seed, v.Statuses)
		if r.Seed != 0 {
			v.Seed = strconv.FormatUint(r.Seed, 10)
		}
	}
	return v
}

// runSummary is the line after a run: how it ended, how many steps passed, failed or were left out, and the seed.
// Before: only "passed · seed 7", so a failed run did not say how much of it had failed.
func runSummary(status string, seed uint64, steps map[string]string) string {
	var passed, failed, left int
	for _, st := range steps {
		switch st {
		case "passed":
			passed++
		case "failed", "error", "aborted":
			failed++
		case "skipped", "cancelled":
			left++
		}
	}
	var counts []string
	if passed > 0 {
		counts = append(counts, fmt.Sprintf("%d passed", passed))
	}
	if failed > 0 {
		counts = append(counts, fmt.Sprintf("%d failed", failed))
	}
	if left > 0 {
		counts = append(counts, fmt.Sprintf("%d not run", left))
	}
	if len(counts) == 0 {
		return fmt.Sprintf("%s · seed %d", status, seed)
	}
	return fmt.Sprintf("%s · %s · seed %d", status, strings.Join(counts, ", "), seed)
}

func varsOf(o *openFlow) VarsView {
	f := vm.ScanFlow(o.doc.Draft())
	return VarsView{Set: nonNil(f.Set), Secrets: nonNil(f.Secrets), Unset: nonNil(f.Unset), Values: varValuesOf(o.run.Vars()), Uses: varUsesOf(f)}
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

// stepMessagesOf joins each message row to the feed fields of the same message in the record, so the output dock
// shows the same feed as the request's response pane. Before, the dock listed bare text with no time, type or search.
func stepMessagesOf(rows []vm.MessageRow, d vm.NodeDetail) []StepMessage {
	var feed []MessageView
	if d.Record != nil {
		feed = messagesOf(d.Record.Messages)
	}
	out := make([]StepMessage, 0, len(rows))
	for i, r := range rows {
		m := StepMessage{Direction: r.Direction, Text: r.Text, Lang: r.Lang, Body: r.Text}
		if i < len(feed) {
			f := feed[i]
			m.At, m.Body, m.Key, m.Kind, m.Type = f.At, f.Body, f.Key, f.Kind, f.Type
		}
		out = append(out, m)
	}
	return out
}

// stepTable is the dataset table of a step's record; nil for any other step.
// Before: a dataset step's rows showed in the dock as a block of JSON.
func stepTable(d vm.NodeDetail) *TableView {
	if d.Record == nil {
		return nil
	}
	return tableOf(d.Record)
}
