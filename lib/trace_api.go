// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"strings"

	"github.com/githugt/flowi/lib/analysis"
	"github.com/githugt/flowi/lib/report"
)

// TraceValue says which step of a saved flow makes a value (a variable, an input or a step's output) and which steps
// read it. It works from the flow as written, so it needs no run.
func (w *Workspace) TraceValue(t FlowTarget, name string) (analysis.ValueTrace, error) {
	_, f, err := w.flowFor(t)
	if err != nil {
		return analysis.ValueTrace{}, err
	}
	return analysis.Trace(f, name), nil
}

// ValueHop is the value at one step of a trace in a finished run: what the step made, or what it read.
type ValueHop struct {
	Node  string `json:"node"`
	Role  string `json:"role"` // "made" or "read"
	Value any    `json:"value,omitempty"`
	// Known is false when the run has no value for the hop: the step did not run, or read the variable before
	// anything set it, or the value is part of a response body the report does not keep.
	Known bool `json:"known"`
}

// TraceValueInRun is TraceValue with the values a finished run had at each step. The values are as the run
// reported them: masked.
func (w *Workspace) TraceValueInRun(t FlowTarget, name string, run report.Run) (analysis.ValueTrace, []ValueHop, error) {
	trace, err := w.TraceValue(t, name)
	if err != nil {
		return analysis.ValueTrace{}, nil, err
	}
	return trace, TraceHops(trace, run), nil
}

// TraceHops finds the value at each maker and reader of trace in run. Steps are taken in the order they started:
// a reader sees the last value written by a step that started before it.
func TraceHops(trace analysis.ValueTrace, run report.Run) []ValueHop {
	steps := flattenReport(run.Nodes, nil)
	var hops []ValueHop
	for _, id := range trace.Producers {
		hops = append(hops, madeHop(trace, steps, id))
	}
	for _, id := range trace.Readers {
		hops = append(hops, readHop(trace, steps, id))
	}
	return hops
}

// flattenReport lists the nodes of a report and everything inside them in the order they started.
func flattenReport(list []report.Node, out []report.Node) []report.Node {
	for _, n := range list {
		out = append(out, n)
		for _, h := range n.Hooks {
			out = flattenReport(h.Nodes, out)
		}
		for _, l := range n.Lanes {
			out = flattenReport(l.Nodes, out)
		}
	}
	return out
}

// lastIndex is the position of the last instance of step id (a step in a batch ran once per item), or -1.
func lastIndex(steps []report.Node, id string) int {
	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i].ID == id {
			return i
		}
	}
	return -1
}

func madeHop(trace analysis.ValueTrace, steps []report.Node, id string) ValueHop {
	hop := ValueHop{Node: id, Role: "made"}
	i := lastIndex(steps, id)
	if i < 0 {
		return hop
	}
	if trace.Kind == analysis.TraceVariable {
		hop.Value, hop.Known = writtenBy(steps[i], tracedVariable(trace))
		return hop
	}
	hop.Value, hop.Known = valueAfter(trace, steps, i+1)
	return hop
}

func readHop(trace analysis.ValueTrace, steps []report.Node, id string) ValueHop {
	hop := ValueHop{Node: id, Role: "read"}
	if i := lastIndex(steps, id); i >= 0 {
		hop.Value, hop.Known = valueAfter(trace, steps, i)
	}
	return hop
}

// valueAfter is the traced value as the first n steps left it.
func valueAfter(trace analysis.ValueTrace, steps []report.Node, n int) (any, bool) {
	switch trace.Kind {
	case analysis.TraceVariable:
		for i := n - 1; i >= 0; i-- {
			if v, ok := writtenBy(steps[i], tracedVariable(trace)); ok {
				return v, true
			}
		}
	case analysis.TraceOutput:
		return outputOf(trace, steps)
	}
	return nil, false
}

func tracedVariable(trace analysis.ValueTrace) string { return strings.TrimPrefix(trace.Name, "vars.") }

// writtenBy is the last value step wrote to variable name.
func writtenBy(step report.Node, name string) (any, bool) {
	for i := len(step.Wrote) - 1; i >= 0; i-- {
		if step.Wrote[i].Name == name {
			return step.Wrote[i].Value, true
		}
	}
	return nil, false
}

// outputOf walks a traced step output (nodes.login.token) down the outputs the report kept for its step.
func outputOf(trace analysis.ValueTrace, steps []report.Node) (any, bool) {
	path := strings.Split(trace.Name, ".")
	if len(path) < 3 {
		return nil, false
	}
	i := lastIndex(steps, path[1])
	if i < 0 {
		return nil, false
	}
	var v any = steps[i].Outputs
	for _, key := range path[2:] {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, false
		}
		if v, ok = m[key]; !ok {
			return nil, false
		}
	}
	return v, true
}
