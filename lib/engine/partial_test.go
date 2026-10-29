// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"testing"

	"github.com/githugt/flowi-lib/model"
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

func TestRunFlow_Given_prior_vars_Then_the_chosen_nodes_read_them(t *testing.T) {
	// Arrange
	f := newFixture(t)
	fl := f.addFlow("partial-vars",
		setNode("a", "vars.token", "abc"),
		assertNode("b", `vars.token == "abc"`))
	in := f.runInput(fl)
	in.Only = []string{"b"}
	in.PriorVars = map[string]any{"token": "abc"}

	// Act
	events, res := runFlow(context.Background(), t, f.deps(), in)

	// Assert
	b, ok := finishedNode(events, "b")
	if res.Status != StatusPassed || !ok || b.Status != StatusPassed {
		t.Fatalf("run %s, b %+v (%v): a set node left out of the run must still have its variable", res.Status, b, res.Diags)
	}
}

func TestRunFlow_Given_a_loop_Then_every_iteration_reads_the_prior_outputs(t *testing.T) {
	// Arrange
	f := newFixture(t)
	fl := f.addFlow("loop-prior",
		flowNode("a", "note", "text", "never runs"),
		assertNode("b", "nodes.a.n == 7"),
		setNode("c", "vars.i", "{{(vars.i ?? 0) + 1}}"))
	fl.Run = &model.RunTill{Until: "vars.i >= 2", MaxIterations: 5}
	in := f.runInput(fl)
	in.Only = []string{"b", "c"}
	in.Prior = map[string]map[string]any{"a": {"n": 7}}

	// Act
	_, res := runFlow(context.Background(), t, f.deps(), in)

	// Assert
	if res.Status != StatusPassed {
		t.Fatalf("run %s (%v): the loop ignored the outputs of the nodes left out", res.Status, res.Diags)
	}
}

func TestRunFlow_should_report_the_variables_the_flow_ended_with_masked(t *testing.T) {
	// Arrange
	f := newFixture(t)
	fl := f.addFlow("final-vars",
		setNode("a", "vars.count", "{{1 + 1}}"),
		setNode("b", "vars.label", "done"))

	// Act
	_, res := runFlow(context.Background(), t, f.deps(), f.runInput(fl))

	// Assert
	if res.Report.Vars["label"] != "done" || res.Report.Vars["count"] == nil {
		t.Fatalf("vars %v", res.Report.Vars)
	}
}
