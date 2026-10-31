// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/nodes"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/vault"
)

// flowNode builds a node with params (key, value, …).
func flowNode(id string, t model.NodeType, params ...any) model.Node {
	n := model.Node{ID: id, Type: t}
	for i := 0; i < len(params); i += 2 {
		n.Params = append(n.Params, model.Field{Key: params[i].(string), Value: params[i+1]})
	}
	return n
}

// setValues is a set node that writes the given values.
func setValues(id string, values ...model.SetValue) model.Node {
	return model.Node{ID: id, Type: "set", Values: values}
}

func (f *fixture) runInput(fl model.Flow) RunInput {
	return RunInput{Collection: f.coll(), Flow: fl, Env: "local"}
}

// runFlow runs a flow to the end and returns its events and result.
func runFlow(ctx context.Context, t *testing.T, d Deps, in RunInput) ([]Event, RunResult) {
	t.Helper()
	run := StartRun(ctx, d, in)
	var events []Event
	for ev := range run.Events() {
		events = append(events, ev)
	}
	return events, run.Wait()
}

func eventNames(events []Event) []string {
	out := make([]string, len(events))
	for i, ev := range events {
		out[i] = ev.EventName()
	}
	return out
}

// withRunners adds test node types to the registry.
func withRunners(d Deps, extra map[model.NodeType]nodes.Runner) Deps {
	d.Nodes = func(t model.NodeType) (nodes.Runner, bool) {
		if r, ok := extra[t]; ok {
			return r, true
		}
		return nodes.Lookup(t)
	}
	return d
}

func finishedNode(events []Event, path string) (NodeFinished, bool) {
	for _, ev := range events {
		if nf, ok := ev.(NodeFinished); ok && nf.Path == path {
			return nf, true
		}
	}
	return NodeFinished{}, false
}

func TestStartRun_should_emit_started_node_events_and_finished_in_order(t *testing.T) {
	f := newFixture(t)
	fl := f.addFlow("simple",
		flowNode("a", "set", "target", "vars.x", "value", "{{1}}"),
		flowNode("b", "log", "value", "x={{vars.x}}"),
		flowNode("c", "note", "text", "done {{vars.x}}"))

	events, res := runFlow(context.Background(), t, f.deps(), f.runInput(fl))

	want := []string{"RunStarted", "NodeStarted", "NodeFinished", "NodeStarted", "Log", "NodeFinished", "NodeStarted", "NodeFinished", "RunFinished"}
	if got := eventNames(events); len(got) != len(want) {
		t.Fatalf("events %v", got)
	}
	for i, name := range eventNames(events) {
		if name != want[i] {
			t.Fatalf("event %d is %s, want %s (%v)", i, name, want[i], eventNames(events))
		}
	}
	started := events[0].(RunStarted)
	if started.Kind != "flow" || started.FlowID != string(fl.ID) || started.FlowPath != "simple" || started.Env != "local" {
		t.Fatalf("started %+v", started)
	}
	if log := events[4].(Log); log.Path != "b" || log.Message != "x=1" {
		t.Fatalf("log %+v", log)
	}
	c, _ := finishedNode(events, "c")
	if res.Status != StatusPassed || c.Status != StatusPassed || c.Outputs["text"] != "done 1" || c.Type != "note" {
		t.Fatalf("result %+v node %+v", res, c)
	}
	if last := events[len(events)-1].(RunFinished); last.Status != StatusPassed {
		t.Fatalf("finished %+v", last)
	}
}

func TestStartRun_Given_when_false_Then_node_skipped_and_has_no_outputs(t *testing.T) {
	f := newFixture(t)
	skipped := setValues("a", model.SetValue{Name: "one", Value: "{{1}}"})
	skipped.When = "env.base_url == ''"
	fl := f.addFlow("skip", skipped, flowNode("b", "note", "text", "{{nodes.a ?? 'none'}}"))

	events, res := runFlow(context.Background(), t, f.deps(), f.runInput(fl))

	a, _ := finishedNode(events, "a")
	b, _ := finishedNode(events, "b")
	if res.Status != StatusPassed || a.Status != StatusSkipped || a.Outputs != nil || b.Outputs["text"] != "none" {
		t.Fatalf("a %+v b %+v", a, b)
	}
}

func TestStartRun_Given_failed_node_Then_run_failed_and_later_nodes_skipped(t *testing.T) {
	f := newFixture(t)
	fl := f.addFlow("fail", flowNode("a", "assert", "check", "1 == 2"), flowNode("b", "note", "text", "never"))

	events, res := runFlow(context.Background(), t, f.deps(), f.runInput(fl))

	if b, _ := finishedNode(events, "b"); b.Status != StatusSkipped || b.Outputs != nil || res.Status != StatusFailed || !hasCode(res.Diags, diag.CodeRunAssertFailed) {
		t.Fatalf("result %+v events %v", res, eventNames(events))
	}
}

func TestStartRun_Given_node_timeout_Then_FLW_RUN_007_failed(t *testing.T) {
	f := newFixture(t)
	block := func(ctx context.Context, _ model.Node, _ *nodes.State) nodes.Outcome {
		<-ctx.Done()
		return nodes.Outcome{Status: nodes.StatusCancelled}
	}
	n := flowNode("slow", "block")
	n.Timeout = time.Millisecond
	fl := f.addFlow("timeout", n)

	events, res := runFlow(context.Background(), t, withRunners(f.deps(), map[model.NodeType]nodes.Runner{"block": block}), f.runInput(fl))

	slow, _ := finishedNode(events, "slow")
	if res.Status != StatusFailed || slow.Status != StatusFailed || !hasCode(slow.Diags, diag.CodeRunNodeTimeout) {
		t.Fatalf("result %+v node %+v", res, slow)
	}
}

func TestStartRun_should_mask_secret_outputs_in_NodeFinished(t *testing.T) {
	f := newFixture(t)
	fl := f.addFlow("secret", setValues("a", model.SetValue{Name: "out", Value: "pw={{secret.password}}"}))

	events, res := runFlow(context.Background(), t, f.deps(), f.runInput(fl))

	a, _ := finishedNode(events, "a")
	if res.Status != StatusPassed || a.Outputs["out"] != vault.Mask {
		t.Fatalf("result %+v outputs %v", res, a.Outputs)
	}
}

func TestStartRun_Given_unknown_environment_Then_failed_with_FLW_REQ_004(t *testing.T) {
	f := newFixture(t)
	fl := f.addFlow("env", flowNode("a", "note", "text", "x"))
	in := f.runInput(fl)
	in.Env = "nowhere"

	_, res := runFlow(context.Background(), t, f.deps(), in)

	if res.Status != StatusFailed || !hasCode(res.Diags, diag.CodeRequestEnvUnknown) {
		t.Fatalf("result %+v", res)
	}
}

func TestStartRun_delay_should_wait_on_the_run_clock(t *testing.T) {
	f := newFixture(t)
	clock := newFakeClock()
	d := f.deps()
	d.Clock, d.After = clock.Now, clock.After
	fl := f.addFlow("wait", flowNode("pause", "delay", "duration", "2s"), flowNode("after", "note", "text", "done"))

	run := StartRun(context.Background(), d, f.runInput(fl))
	clock.WaitForTimers(t, 1)
	clock.Advance(2 * time.Second)
	var events []Event
	for ev := range run.Events() {
		events = append(events, ev)
	}
	res := run.Wait()

	pause, _ := finishedNode(events, "pause")
	if res.Status != StatusPassed || pause.Duration != 2*time.Second {
		t.Fatalf("result %+v pause %+v", res, pause)
	}
}

func TestStartRun_Given_a_stop_node_Then_the_rest_is_skipped_and_the_run_passes(t *testing.T) {
	f := newFixture(t)
	fl := f.addFlow("stop", flowNode("a", "note", "text", "first"), flowNode("halt", "stop", "message", "enough"), flowNode("b", "note", "text", "never"))

	events, res := runFlow(context.Background(), t, f.deps(), f.runInput(fl))

	a, _ := finishedNode(events, "a")
	b, _ := finishedNode(events, "b")
	if res.Status != StatusPassed || a.Status != StatusPassed || b.Status != StatusSkipped {
		t.Fatalf("result %+v, a %+v, b %+v", res, a, b)
	}
}

// A stop inside a branch of a condition ends the whole run, not only the branch.
func TestStartRun_Given_a_stop_in_a_branch_Then_the_steps_after_the_condition_are_skipped(t *testing.T) {
	f := newFixture(t)
	cond := model.Node{ID: "c", Type: "condition", Params: []model.Field{{Key: "check", Value: "true"}}, Then: []model.Node{flowNode("halt", "stop")}}
	fl := f.addFlow("stopbranch", cond, flowNode("after", "note", "text", "never"))

	events, res := runFlow(context.Background(), t, f.deps(), f.runInput(fl))

	after, _ := finishedNode(events, "after")
	if res.Status != StatusPassed || after.Status != StatusSkipped {
		t.Fatalf("result %+v, after %+v", res, after)
	}
}

func TestStartRun_Given_a_stop_as_failed_Then_the_run_fails_with_the_message(t *testing.T) {
	f := newFixture(t)
	fl := f.addFlow("stopfail", flowNode("halt", "stop", "status", "failed", "message", "no token"), flowNode("b", "note", "text", "never"))

	events, res := runFlow(context.Background(), t, f.deps(), f.runInput(fl))

	b, _ := finishedNode(events, "b")
	if res.Status != StatusFailed || !hasCode(res.Diags, diag.CodeRunStopped) || b.Status != StatusSkipped {
		t.Fatalf("result %+v, b %+v", res, b)
	}
}
