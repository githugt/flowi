// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"testing"

	"github.com/githugt/flowi/lib/analysis"
	"github.com/githugt/flowi/lib/report"
)

func tracedRun() report.Run {
	return report.Run{Nodes: []report.Node{
		{ID: "login", Type: "request", Outputs: map[string]any{"token": "t-1", "status": 200}},
		{ID: "grab", Type: "set", Wrote: []report.VarWrite{{Name: "token", Value: "t-1"}}},
		{ID: "show", Type: "log"},
		{ID: "refresh", Type: "set", Wrote: []report.VarWrite{{Name: "token", Value: "t-2"}}},
		{ID: "use", Type: "log"},
	}}
}

func hopOf(hops []ValueHop, node string) (ValueHop, bool) {
	for _, h := range hops {
		if h.Node == node {
			return h, true
		}
	}
	return ValueHop{}, false
}

func TestTraceHops_should_give_a_reader_the_value_the_variable_had_when_it_ran(t *testing.T) {
	trace := analysis.ValueTrace{Name: "vars.token", Kind: analysis.TraceVariable, Producers: []string{"grab", "refresh"}, Readers: []string{"show", "use"}}

	hops := TraceHops(trace, tracedRun())

	for node, want := range map[string]any{"grab": "t-1", "refresh": "t-2", "show": "t-1", "use": "t-2"} {
		h, ok := hopOf(hops, node)
		if !ok || !h.Known || h.Value != want {
			t.Errorf("%s: %+v, want %v", node, h, want)
		}
	}
}

func TestTraceHops_should_read_a_step_output_for_its_maker_and_readers(t *testing.T) {
	trace := analysis.ValueTrace{Name: "nodes.login.token", Kind: analysis.TraceOutput, Producers: []string{"login"}, Readers: []string{"grab"}}

	hops := TraceHops(trace, tracedRun())

	for _, node := range []string{"login", "grab"} {
		if h, _ := hopOf(hops, node); !h.Known || h.Value != "t-1" {
			t.Errorf("%s: %+v", node, h)
		}
	}
}

func TestTraceHops_Given_a_step_that_did_not_run_Then_the_value_is_not_known(t *testing.T) {
	trace := analysis.ValueTrace{Name: "vars.token", Kind: analysis.TraceVariable, Producers: []string{"grab"}, Readers: []string{"ghost"}}

	hops := TraceHops(trace, tracedRun())

	if h, _ := hopOf(hops, "ghost"); h.Known {
		t.Fatalf("a step that never ran has no value: %+v", h)
	}
}

func TestTraceHops_Given_a_variable_read_before_anything_wrote_it_Then_the_value_is_not_known(t *testing.T) {
	trace := analysis.ValueTrace{Name: "vars.token", Kind: analysis.TraceVariable, Producers: []string{"grab"}, Readers: []string{"login"}}

	hops := TraceHops(trace, tracedRun())

	if h, _ := hopOf(hops, "login"); h.Known {
		t.Fatalf("%+v", h)
	}
}
