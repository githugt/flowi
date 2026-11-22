// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/nodes"
	"github.com/githugt/flowi/lib/model"
)

// when is a condition that runs then when check holds.
func when(id, check string, then ...model.Node) model.Node {
	return model.Node{ID: id, Type: "condition", Params: []model.Field{{Key: "check", Value: check}}, Then: then}
}

func loopControl(id, action string) model.Node {
	return flowNode(id, "loop_control", "action", action, "message", "enough")
}

func timesParams(count, concurrency int64) []model.Field {
	return []model.Field{{Key: "times", Value: count}, {Key: "concurrency", Value: concurrency}}
}

func TestLoopControl_Given_next_in_a_batch_Then_that_item_ends_passed_and_the_others_go_on(t *testing.T) {
	// Before: the only way out of an iteration was stop, which ended the whole run.
	f := newFixture(t)
	p := newProbe()
	body := []model.Node{probeNode("a"), when("skipit", "index == 1", loopControl("next", "next")), probeNode("b")}
	fl := f.addFlow("next", batchOf("users", timesParams(3, 1), body...), probeNode("after"))

	events, res := runFlow(testCtx(t), t, p.deps(f.deps()), f.runInput(fl))

	users, _ := finishedNode(events, "users")
	skipped, _ := finishedNode(events, "users[1]/b")
	if res.Status != StatusPassed || users.Status != StatusPassed || users.Outputs["left"] != false || users.Outputs["passed"] != 3 {
		t.Fatalf("run %s users %+v", res.Status, users)
	}
	if !p.started("users[0]/b") || p.started("users[1]/b") || !p.started("users[2]/b") || !p.started("after") || skipped.Status != StatusSkipped {
		t.Fatalf("order %v, b of item 1 %+v", p.order, skipped)
	}
}

func TestLoopControl_Given_leave_in_a_sequential_batch_Then_the_next_items_never_start_and_the_batch_passes(t *testing.T) {
	// Before: there was no break; a loop always went through every item.
	f := newFixture(t)
	p := newProbe()
	body := []model.Node{probeNode("a"), when("found", "index == 1", loopControl("leave", "leave")), probeNode("b")}
	fl := f.addFlow("leave", batchOf("users", timesParams(4, 1), body...), probeNode("after"))

	events, res := runFlow(testCtx(t), t, p.deps(f.deps()), f.runInput(fl))

	users, _ := finishedNode(events, "users")
	results, _ := users.Outputs["results"].([]any)
	if res.Status != StatusPassed || users.Status != StatusPassed || users.Outputs["left"] != true || len(results) != 4 {
		t.Fatalf("run %s users %+v", res.Status, users)
	}
	if !p.started("users[0]/b") || p.started("users[1]/b") || p.started("users[2]/a") || p.started("users[3]/a") || !p.started("after") {
		t.Fatalf("order %v", p.order)
	}
	if third := results[2].(map[string]any); third["status"] != "skipped" {
		t.Fatalf("an item that never started should be skipped: %v", third)
	}
}

func TestLoopControl_Given_leave_in_a_concurrent_batch_Then_running_items_finish_and_no_new_one_starts(t *testing.T) {
	f := newFixture(t)
	p := newProbe()
	left := make(chan struct{})
	p.behave["gate"] = func(ctx context.Context, s *nodes.State) nodes.Outcome {
		if s.Path == "users[1]/gate" { // item 1 is running when item 0 leaves
			select {
			case <-left:
			case <-ctx.Done():
			}
		}
		return nodes.Outcome{}
	}
	realControl, _ := nodes.Lookup("loop_control")
	leaver := func(ctx context.Context, n model.Node, s *nodes.State) nodes.Outcome {
		defer close(left)
		return realControl(ctx, n, s)
	}
	d := withRunners(f.deps(), map[model.NodeType]nodes.Runner{"probe": p.run, "leaver": leaver})
	body := []model.Node{when("first", "index == 0", model.Node{ID: "out", Type: "leaver", Params: []model.Field{{Key: "action", Value: "leave"}}}), probeNode("gate"), probeNode("b")}
	fl := f.addFlow("leave-concurrent", batchOf("users", timesParams(4, 2), body...))

	events, res := runFlow(testCtx(t), t, d, f.runInput(fl))

	users, _ := finishedNode(events, "users")
	if res.Status != StatusPassed || users.Outputs["left"] != true {
		t.Fatalf("run %s users %+v", res.Status, users)
	}
	if !p.started("users[1]/b") || p.started("users[0]/gate") || p.started("users[2]/gate") || p.started("users[3]/gate") {
		t.Fatalf("item 1 was running and must finish; items 2 and 3 must not start: %v", p.order)
	}
}

func TestLoopControl_Given_next_in_run_till_Then_the_round_counts_and_until_is_still_asked(t *testing.T) {
	f := newFixture(t)
	p := newProbe()
	loop := model.Node{ID: "poll", Type: "run_till", Params: []model.Field{{Key: "until", Value: "true"}},
		Body: []model.Node{when("skipit", "true", loopControl("next", "next")), probeNode("b")}}
	fl := f.addFlow("till-next", loop)

	events, res := runFlow(testCtx(t), t, p.deps(f.deps()), f.runInput(fl))

	poll, _ := finishedNode(events, "poll")
	if res.Status != StatusPassed || poll.Outputs["met"] != true || poll.Outputs["left"] != false || poll.Outputs["iterations"] != 1 || p.started("poll[0]/b") {
		t.Fatalf("run %s poll %+v order %v", res.Status, poll, p.order)
	}
}

func TestLoopControl_Given_next_in_every_round_Then_run_till_still_ends_at_its_limit(t *testing.T) {
	f := newFixture(t)
	loop := model.Node{ID: "poll", Type: "run_till", Params: []model.Field{{Key: "until", Value: "false"}, {Key: "max", Value: int64(3)}},
		Body: []model.Node{loopControl("next", "next")}}
	fl := f.addFlow("till-limit", loop)

	events, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	poll, _ := finishedNode(events, "poll")
	if poll.Outputs["iterations"] != 3 || !hasCode(poll.Diags, diag.CodeRunTillLimit) || res.Status != StatusFailed {
		t.Fatalf("run %s poll %+v: a next must not stop the limit from counting", res.Status, poll)
	}
}

func TestLoopControl_Given_leave_in_run_till_Then_the_loop_ends_as_met_without_asking_until(t *testing.T) {
	f := newFixture(t)
	p := newProbe()
	rounds := 0
	p.behave["count"] = func(_ context.Context, s *nodes.State) nodes.Outcome {
		rounds++
		s.Vars.Set("n", rounds)
		return nodes.Outcome{}
	}
	loop := model.Node{ID: "poll", Type: "run_till", Params: []model.Field{{Key: "until", Value: "false"}, {Key: "max", Value: int64(9)}},
		Body: []model.Node{probeNode("count"), when("enough", "vars.n == 2", loopControl("leave", "leave")), probeNode("tail")}}
	fl := f.addFlow("till-leave", loop, probeNode("after"))

	events, res := runFlow(testCtx(t), t, p.deps(f.deps()), f.runInput(fl))

	poll, _ := finishedNode(events, "poll")
	if res.Status != StatusPassed || poll.Outputs["iterations"] != 2 || poll.Outputs["met"] != true || poll.Outputs["left"] != true || hasCode(poll.Diags, diag.CodeRunTillLimit) {
		t.Fatalf("run %s poll %+v", res.Status, poll)
	}
	if !p.started("poll[0]/tail") || p.started("poll[1]/tail") || !p.started("after") {
		t.Fatalf("the rest of the leaving round is skipped, the steps after the loop run: %v", p.order)
	}
}

func TestLoopControl_Given_a_nested_loop_Then_it_acts_on_the_nearest_loop_only(t *testing.T) {
	f := newFixture(t)
	p := newProbe()
	inner := batchOf("inner", timesParams(3, 1), when("skipit", "index == 1", loopControl("next", "next")), probeNode("x"))
	outer := batchOf("outer", timesParams(2, 1), inner, probeNode("tail"))
	fl := f.addFlow("nested", outer)

	_, res := runFlow(testCtx(t), t, p.deps(f.deps()), f.runInput(fl))

	if res.Status != StatusPassed || !p.started("outer[0]/tail") || !p.started("outer[1]/tail") {
		t.Fatalf("the outer turns must go on to their tail: %v", p.order)
	}
	if !p.started("outer[1]/inner[0]/x") || p.started("outer[1]/inner[1]/x") || !p.started("outer[1]/inner[2]/x") {
		t.Fatalf("only the inner item 1 is cut short: %v", p.order)
	}
}

func TestLoopControl_Given_next_in_a_parallel_branch_Then_the_rest_of_the_item_is_skipped(t *testing.T) {
	f := newFixture(t)
	p := newProbe()
	par := parallelOf("both", "all",
		model.Branch{ID: "one", Nodes: []model.Node{loopControl("next", "next")}},
		model.Branch{ID: "two", Nodes: []model.Node{probeNode("x")}})
	fl := f.addFlow("par", batchOf("users", timesParams(2, 1), par, probeNode("tail")))

	_, res := runFlow(testCtx(t), t, p.deps(f.deps()), f.runInput(fl))

	if res.Status != StatusPassed || p.started("users[0]/tail") || p.started("users[1]/tail") {
		t.Fatalf("run %s: steps after the parallel must be skipped: %v", res.Status, p.order)
	}
}

func TestLoopControl_Given_a_skipped_step_Then_it_does_not_run(t *testing.T) {
	f := newFixture(t)
	p := newProbe()
	skipped := loopControl("next", "leave")
	skipped.Skipped = true
	fl := f.addFlow("skipped", batchOf("users", timesParams(2, 1), skipped, probeNode("tail")))

	events, res := runFlow(testCtx(t), t, p.deps(f.deps()), f.runInput(fl))

	users, _ := finishedNode(events, "users")
	if res.Status != StatusPassed || users.Outputs["left"] != false || !p.started("users[0]/tail") || !p.started("users[1]/tail") {
		t.Fatalf("run %s users %+v order %v", res.Status, users, p.order)
	}
}

func TestLoopControl_Given_a_step_outside_any_loop_Then_it_fails_and_the_rest_is_skipped(t *testing.T) {
	// Before: validate refuses it, but the engine is public: a flow that skipped validation must not pass silently.
	f := newFixture(t)
	p := newProbe()
	fl := f.addFlow("outside", loopControl("next", "next"), probeNode("after"))

	events, res := runFlow(testCtx(t), t, p.deps(f.deps()), f.runInput(fl))

	step, _ := finishedNode(events, "next")
	if res.Status != StatusFailed || !hasCode(step.Diags, diag.CodeNodeLoopControl) || p.started("after") {
		t.Fatalf("run %s step %+v", res.Status, step)
	}
}

func TestLoopControl_Given_a_subflow_inside_a_loop_Then_a_step_in_the_subflow_is_outside_a_loop(t *testing.T) {
	// A subflow is its own flow: its steps do not end the caller's loop.
	f := newFixture(t)
	child := f.addFlow("child", loopControl("next", "next"))
	call := model.Node{ID: "call", Type: "subflow", Subflow: &model.SubflowCall{Flow: model.Ref{ID: child.ID, Path: child.Path}}}
	fl := f.addFlow("parent", batchOf("users", timesParams(1, 1), call))

	events, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	step, _ := finishedNode(events, "users[0]/call/next")
	if res.Status != StatusFailed || !hasCode(step.Diags, diag.CodeNodeLoopControl) {
		t.Fatalf("run %s step %+v", res.Status, step)
	}
}
