// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"testing"
)

func TestRunFlow_Given_only_Then_runs_just_those_nodes_with_prior_outputs(t *testing.T) {
	f := newFixture(t)
	fl := f.addFlow("partial",
		flowNode("a", "note", "text", "never runs"),
		assertNode("b", "nodes.a.n == 7"))
	in := f.runInput(fl)
	in.Only = []string{"b"}
	in.Prior = map[string]map[string]any{"a": {"n": 7}}

	events, res := runFlow(context.Background(), t, f.deps(), in)

	if _, ran := finishedNode(events, "a"); ran {
		t.Fatalf("node a ran: %v", eventNames(events))
	}
	b, ok := finishedNode(events, "b")
	if res.Status != StatusPassed || !ok || b.Status != StatusPassed {
		t.Fatalf("run %s, b %+v (%v)", res.Status, b, res.Diags)
	}
}

func TestRunFlow_Given_only_unknown_id_Then_nothing_runs_and_run_passes(t *testing.T) {
	f := newFixture(t)
	fl := f.addFlow("partial-none", flowNode("a", "note", "text", "x"))
	in := f.runInput(fl)
	in.Only = []string{"zzz"}

	events, res := runFlow(context.Background(), t, f.deps(), in)

	if _, ran := finishedNode(events, "a"); ran || res.Status != StatusPassed {
		t.Fatalf("run %s: %v", res.Status, eventNames(events))
	}
}
