// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/report"
)

// reportBuilder turns a run's events into its report tree (design §7.7),
// so the live view, the saved report and digests all tell the same story.
// It is fed by runner.emit; mu guards everything.
type reportBuilder struct {
	clock func() time.Time

	mu    sync.Mutex
	run   report.Run
	roots []*buildNode
	nodes map[string]*buildNode // by instance path
}

// buildNode is a node while the run builds it; children hang off lanes
// or hooks in the order they started.
type buildNode struct {
	node  report.Node
	lanes []*buildLane
	hooks []*buildHook
}

type buildLane struct {
	label string
	nodes []*buildNode
}

type buildHook struct {
	label string // phase:level:index — the lane label its nodes carry
	hook  report.Hook
	nodes []*buildNode
}

func newReportBuilder(clock func() time.Time, flowName, collection string) *reportBuilder {
	return &reportBuilder{clock: clock, run: report.Run{FlowName: flowName, Collection: collection}, nodes: map[string]*buildNode{}}
}

// add applies one event.
func (b *reportBuilder) add(ev Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch e := ev.(type) {
	case RunStarted:
		b.run.ID, b.run.Kind, b.run.FlowID, b.run.FlowPath, b.run.Env, b.run.Seed, b.run.Started = e.RunID, e.Kind, e.FlowID, e.FlowPath, e.Env, e.Seed, e.At
		b.run.Overlay = e.Overlay
		b.run.Status = "running"
	case NodeStarted:
		b.nodeStarted(e)
	case NodeFinished:
		b.nodeFinished(e)
	case HookStarted:
		b.hookStarted(e)
	case HookFinished:
		b.hookFinished(e)
	case RequestSent:
		b.requestSent(e)
	case ResponseReceived:
		b.responseReceived(e)
	case Warning:
		b.run.Warnings = append(b.run.Warnings, reportDiag(e.Diag))
	case RunAborted:
		b.run.Aborted = &report.Abort{Reason: e.Reason, Grace: e.Grace, Inflight: e.Inflight}
	case RunFinished:
		b.run.Status, b.run.Duration = string(e.Status), e.Duration
	}
}

func (b *reportBuilder) nodeStarted(e NodeStarted) {
	bn := &buildNode{node: report.Node{Path: e.Path, ID: e.Node, Type: e.Type, Name: e.Name, Status: "running", Started: b.clock()}}
	b.nodes[e.Path] = bn
	parent, ok := b.nodes[e.Parent]
	switch {
	case !ok:
		b.roots = append(b.roots, bn)
	case isHookLane(e.Lane):
		h := parent.hook(e.Lane)
		h.nodes = append(h.nodes, bn)
	default:
		l := parent.lane(e.Lane)
		l.nodes = append(l.nodes, bn)
	}
}

func (b *reportBuilder) nodeFinished(e NodeFinished) {
	bn, ok := b.nodes[e.Path]
	if !ok {
		return
	}
	n := &bn.node
	n.Status, n.Handled, n.Duration, n.Outputs, n.Diagnostics = string(e.Status), e.Handled, e.Duration, e.Outputs, reportDiags(e.Diags)
	n.Reason = e.Reason
	if e.Type == "note" && e.Status == StatusPassed {
		text, _ := e.Outputs["text"].(string)
		b.run.Notes = append(b.run.Notes, report.Note{Path: e.Path, Text: text})
	}
}

func (b *reportBuilder) hookStarted(e HookStarted) {
	bn, ok := b.nodes[e.Path]
	if !ok {
		return
	}
	h := bn.hook(hookLabel(e.Phase, e.Level, e.Index))
	h.hook = report.Hook{Phase: e.Phase, Level: e.Level, Step: e.Step, Index: e.Index, Status: "running"}
}

func (b *reportBuilder) hookFinished(e HookFinished) {
	bn, ok := b.nodes[e.Path]
	if !ok {
		return
	}
	h := bn.hook(hookLabel(e.Phase, e.Level, e.Index))
	h.hook.Status, h.hook.Duration, h.hook.Diagnostics = string(e.Status), e.Duration, reportDiags(e.Diags)
}

func (b *reportBuilder) requestSent(e RequestSent) {
	bn, ok := b.nodes[e.Path]
	if !ok || e.Depth != bn.depth() {
		return // a request inside a hook flow belongs to its own node
	}
	bn.node.Attempts = append(bn.node.Attempts, report.Attempt{N: e.Attempt, Method: e.Method, URL: e.URL})
	if e.Attempt == 1 && e.RequestID != "" {
		b.run.Versions = append(b.run.Versions, report.VersionUse{Path: e.Path, RequestID: e.RequestID, RequestPath: e.RequestPath, Version: e.Version, Source: e.VersionSource})
	}
}

func (b *reportBuilder) responseReceived(e ResponseReceived) {
	bn, ok := b.nodes[e.Path]
	if !ok {
		return
	}
	for i := range bn.node.Attempts {
		if a := &bn.node.Attempts[i]; a.N == e.Attempt && a.Status == 0 && a.Error == "" {
			a.Status, a.Error, a.Duration = e.Status, e.Error, e.Duration
			return
		}
	}
}

// snapshot is the report so far. Nodes still running read "running", or
// "aborted" once the run was aborted.
func (b *reportBuilder) snapshot() report.Run {
	b.mu.Lock()
	defer b.mu.Unlock()
	r := b.run
	unfinished := "running"
	if r.Aborted != nil {
		unfinished = string(StatusAborted)
	}
	r.Nodes = buildNodes(b.roots, unfinished)
	r.Warnings = append([]report.Diagnostic(nil), b.run.Warnings...)
	r.Notes = append([]report.Note(nil), b.run.Notes...)
	r.Versions = append([]report.VersionUse(nil), b.run.Versions...)
	return r
}

// find returns the finished node at path, as a report of its own.
func (b *reportBuilder) find(path string) (report.Run, bool) {
	b.mu.Lock()
	bn, ok := b.nodes[path]
	finished := ok && bn.node.Status != "running"
	b.mu.Unlock()
	if !finished {
		return report.Run{}, false
	}
	r := b.snapshot()
	n, _ := r.Find(path)
	r.Nodes, r.Notes, r.Versions = []report.Node{n}, nil, nil
	return r, true
}

func buildNodes(list []*buildNode, unfinished string) []report.Node {
	out := make([]report.Node, 0, len(list))
	for _, bn := range list {
		n := bn.node
		if n.Status == "running" {
			n.Status = unfinished
		}
		n.Lanes, n.Hooks = nil, nil
		for _, l := range sortedLanes(bn.lanes) {
			n.Lanes = append(n.Lanes, report.Lane{Label: l.label, Nodes: buildNodes(l.nodes, unfinished)})
		}
		for _, h := range bn.hooks {
			hook := h.hook
			hook.Nodes = buildNodes(h.nodes, unfinished)
			n.Hooks = append(n.Hooks, hook)
		}
		n.Attempts = append([]report.Attempt(nil), n.Attempts...)
		out = append(out, n)
	}
	return out
}

// sortedLanes puts iteration lanes ([0], [1], …) in index order — they
// start concurrently, so arrival order is arbitrary; named lanes (branches)
// keep their order.
func sortedLanes(lanes []*buildLane) []*buildLane {
	out := slices.Clone(lanes)
	slices.SortStableFunc(out, func(a, b *buildLane) int {
		i, aOK := laneIndex(a.label)
		j, bOK := laneIndex(b.label)
		if !aOK || !bOK {
			return 0
		}
		return i - j
	})
	return out
}

func laneIndex(label string) (int, bool) {
	inner, ok := strings.CutPrefix(label, "[")
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSuffix(inner, "]"))
	return n, err == nil
}

func (bn *buildNode) lane(label string) *buildLane {
	for _, l := range bn.lanes {
		if l.label == label {
			return l
		}
	}
	l := &buildLane{label: label}
	bn.lanes = append(bn.lanes, l)
	return l
}

func (bn *buildNode) hook(label string) *buildHook {
	for _, h := range bn.hooks {
		if h.label == label {
			return h
		}
	}
	h := &buildHook{label: label}
	bn.hooks = append(bn.hooks, h)
	return h
}

// depth is the hook depth of the node's own requests: inside hook lanes
// the path carries one ":"-labelled segment per level.
func (bn *buildNode) depth() int {
	depth := 0
	for _, seg := range strings.Split(bn.node.Path, "/") {
		if isHookLane(seg) {
			depth++
		}
	}
	return depth
}

func isHookLane(label string) bool {
	return strings.HasPrefix(label, "pre:") || strings.HasPrefix(label, "post:")
}

func hookLabel(phase, level string, index int) string {
	return phase + ":" + level + ":" + itoa(index)
}

func reportDiags(l diag.List) []report.Diagnostic {
	if len(l) == 0 {
		return nil
	}
	out := make([]report.Diagnostic, len(l))
	for i, d := range l {
		out[i] = reportDiag(d)
	}
	return out
}

// reportDiag renders a (masked) diagnostic for the report.
func reportDiag(d diag.Diagnostic) report.Diagnostic {
	msg, hint := diag.English().Message(d)
	if hint != "" {
		msg += " (" + hint + ")"
	}
	return report.Diagnostic{Code: string(d.Code), Severity: d.Severity.String(), Message: msg, File: d.File, Line: d.Pos.Line, Column: d.Pos.Column}
}
