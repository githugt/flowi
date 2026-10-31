// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/internal/nodes"
	"github.com/githugt/flowi/lib/model"
)

// drain collects a run's events and result.
func drain(run *Run) ([]Event, RunResult) {
	var events []Event
	for ev := range run.Events() {
		events = append(events, ev)
	}
	return events, run.Wait()
}

func lastEvent[T Event](events []Event) (T, bool) {
	for _, ev := range slices.Backward(events) {
		if e, ok := ev.(T); ok {
			return e, true
		}
	}
	var zero T
	return zero, false
}

func TestRun_Given_cancel_during_delay_Then_cancelled_promptly_without_waiting(t *testing.T) {
	f := newFixture(t)
	clock := newFakeClock()
	d := f.deps()
	d.Clock, d.After = clock.Now, clock.After
	fl := f.addFlow("wait", flowNode("pause", "delay", "duration", "1h"), flowNode("after", "note", "text", "x"))
	ctx, cancel := context.WithCancel(testCtx(t))

	run := StartRun(ctx, d, f.runInput(fl))
	clock.WaitForTimers(t, 1)
	cancel()
	events, res := drain(run)

	pause, _ := finishedNode(events, "pause")
	after, _ := finishedNode(events, "after")
	finished, _ := lastEvent[RunFinished](events)
	if res.Status != StatusCancelled || pause.Status != StatusCancelled || after.Status != StatusSkipped || finished.Status != StatusCancelled {
		t.Fatalf("result %s pause %s after %s finished %+v", res.Status, pause.Status, after.Status, finished)
	}
	if _, ok := lastEvent[RunCancelled](events); !ok {
		t.Fatal("no RunCancelled")
	}
}

func TestRun_Given_cancel_during_batch_concurrency_3_Then_no_new_iterations_and_inflight_cancelled(t *testing.T) {
	f := newFixture(t)
	p := newProbe()
	started := make(chan struct{}, 10)
	p.behave["work"] = func(ctx context.Context, _ *nodes.State) nodes.Outcome {
		started <- struct{}{}
		<-ctx.Done()
		return nodes.Outcome{Status: nodes.StatusCancelled}
	}
	batch := batchOf("each", []model.Field{{Key: "times", Value: int64(10)}, {Key: "concurrency", Value: int64(3)}}, probeNode("work"))
	fl := f.addFlow("batch-cancel", batch)
	ctx, cancel := context.WithCancel(testCtx(t))

	run := StartRun(ctx, p.deps(f.deps()), f.runInput(fl))
	for range 3 {
		<-started
	}
	cancel()
	events, res := drain(run)

	each, _ := finishedNode(events, "each")
	if res.Status != StatusCancelled || each.Status != StatusCancelled || len(p.starts) != 3 {
		t.Fatalf("result %s each %s started %d", res.Status, each.Status, len(p.starts))
	}
	for i := range 3 {
		if w, _ := finishedNode(events, "each["+string(rune('0'+i))+"]/work"); w.Status != StatusCancelled {
			t.Fatalf("iteration %d: %+v", i, w)
		}
	}
}

func TestRun_Given_cancel_during_run_till_interval_Then_cancelled(t *testing.T) {
	f := newFixture(t)
	clock := newFakeClock()
	d := f.deps()
	d.Clock, d.After = clock.Now, clock.After
	loop := model.Node{ID: "poll", Type: "run_till", Params: []model.Field{{Key: "until", Value: "false"}, {Key: "interval", Value: "5s"}},
		Body: []model.Node{flowNode("tick", "note", "text", "x")}}
	fl := f.addFlow("till-cancel", loop)
	ctx, cancel := context.WithCancel(testCtx(t))

	run := StartRun(ctx, d, f.runInput(fl))
	clock.WaitForTimers(t, 1)
	cancel()
	events, res := drain(run)

	poll, _ := finishedNode(events, "poll")
	if res.Status != StatusCancelled || poll.Status != StatusCancelled || poll.Outputs["iterations"] != 1 {
		t.Fatalf("result %s poll %+v", res.Status, poll)
	}
}

func TestRun_Given_node_ignoring_cancel_Then_aborted_after_grace(t *testing.T) {
	f := newFixture(t)
	clock := newFakeClock()
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	running := make(chan string, 2)
	blocker := func(_ context.Context, n model.Node, _ *nodes.State) nodes.Outcome {
		running <- n.ID
		<-release // ignores cancellation, like a stuck call
		return nodes.Outcome{}
	}
	d := withRunners(f.deps(), map[model.NodeType]nodes.Runner{"block": blocker})
	d.Clock, d.After = clock.Now, clock.After
	verify := model.Node{ID: "verify", Type: "block", Needs: []string{"start"}}
	page := model.Node{ID: "form-page-2", Type: "block", Needs: []string{"start"}, Concurrent: true}
	fl := f.addFlow("stuck", flowNode("start", "note", "text", "x"), verify, page)
	ctx, cancel := context.WithCancel(testCtx(t))

	run := StartRun(ctx, d, f.runInput(fl))
	<-running
	<-running
	cancel()
	clock.WaitForTimers(t, 1)
	clock.Advance(60 * time.Second)
	events, res := drain(run)

	names := eventNames(events)
	tail := names[len(names)-3:]
	if !slices.Equal(tail, []string{"RunCancelled", "RunAborted", "RunFinished"}) {
		t.Fatalf("events end with %v", names)
	}
	aborted, _ := lastEvent[RunAborted](events)
	if aborted.Reason != "grace_period_exceeded" || aborted.Grace != 60*time.Second || !slices.Equal(aborted.Inflight, []string{"verify", "form-page-2"}) {
		t.Fatalf("aborted %+v", aborted)
	}
	finished, _ := lastEvent[RunFinished](events)
	if res.Status != StatusAborted || finished.Status != StatusAborted {
		t.Fatalf("result %s finished %+v", res.Status, finished)
	}
}

func TestRun_Given_grace_option_Then_used_instead_of_setting(t *testing.T) {
	f := newFixture(t)
	clock := newFakeClock()
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	running := make(chan struct{}, 1)
	blocker := func(context.Context, model.Node, *nodes.State) nodes.Outcome {
		running <- struct{}{}
		<-release
		return nodes.Outcome{}
	}
	d := withRunners(f.deps(), map[model.NodeType]nodes.Runner{"block": blocker})
	d.Clock, d.After = clock.Now, clock.After
	fl := f.addFlow("stuck2", model.Node{ID: "b", Type: "block"})
	in := f.runInput(fl)
	in.Grace = 5 * time.Second
	ctx, cancel := context.WithCancel(testCtx(t))

	run := StartRun(ctx, d, in)
	<-running
	cancel()
	clock.WaitForTimers(t, 1)
	clock.Advance(5 * time.Second)
	events, res := drain(run)

	aborted, _ := lastEvent[RunAborted](events)
	if res.Status != StatusAborted || aborted.Grace != 5*time.Second {
		t.Fatalf("result %s aborted %+v", res.Status, aborted)
	}
}
