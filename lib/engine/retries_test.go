// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/nodes"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/report"
)

// flaky is a runner that fails its first `failures` calls and then passes, counting the calls.
type flaky struct {
	mu       sync.Mutex
	failures int
	calls    int
}

func (f *flaky) run(context.Context, model.Node, *nodes.State) nodes.Outcome {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.calls <= f.failures {
		return nodes.Outcome{Status: nodes.StatusFailed, Diags: assertDiag()}
	}
	return nodes.Outcome{}
}

func retriedNode(retries int, wait time.Duration) model.Node {
	return model.Node{ID: "a", Type: "request", Retries: retries, RetryWait: wait}
}

func warned(events []Event, code diag.Code) bool {
	for _, ev := range events {
		if w, ok := ev.(Warning); ok && w.Diag.Code == code {
			return true
		}
	}
	return false
}

// Before: a failed request step failed the flow at once; a transient failure needed the whole flow run again.
func TestRunNode_Given_retries_Then_a_step_that_fails_then_passes_passes_and_is_flaky(t *testing.T) {
	f := newFixture(t)
	step := &flaky{failures: 2}
	d := withRunners(f.deps(), map[model.NodeType]nodes.Runner{"request": step.run})
	fl := f.addFlow("flaky", retriedNode(3, 0))

	events, res := runFlow(testCtx(t), t, d, f.runInput(fl))

	nf, _ := finishedNode(events, "a")
	if res.Status != StatusPassed || step.calls != 3 || nf.Status != StatusPassed || nf.Tries != 3 || !warned(events, diag.CodeRunFlaky) {
		t.Fatalf("status %s, calls %d, finished %+v, flaky warning %v", res.Status, step.calls, nf, warned(events, diag.CodeRunFlaky))
	}
	if len(nf.Outcomes) != 1 || nf.Outcomes[0] != model.OutcomeSuccess {
		t.Fatalf("a step that passed on its last try ended as %v", nf.Outcomes)
	}
}

func TestRunNode_Given_retries_that_run_out_Then_the_step_fails_after_its_last_try(t *testing.T) {
	f := newFixture(t)
	step := &flaky{failures: 99}
	d := withRunners(f.deps(), map[model.NodeType]nodes.Runner{"request": step.run})
	fl := f.addFlow("always-fails", retriedNode(2, 0))

	events, res := runFlow(testCtx(t), t, d, f.runInput(fl))

	nf, _ := finishedNode(events, "a")
	if res.Status != StatusFailed || step.calls != 3 || nf.Tries != 3 || warned(events, diag.CodeRunFlaky) {
		t.Fatalf("status %s, calls %d, finished %+v", res.Status, step.calls, nf)
	}
	if len(nf.Outcomes) != 1 || nf.Outcomes[0] != model.OutcomeFailure {
		t.Fatalf("a step that ran out of tries ended as %v", nf.Outcomes)
	}
}

func TestRunNode_Given_no_retries_Then_a_step_runs_once_and_reports_one_try(t *testing.T) {
	f := newFixture(t)
	step := &flaky{failures: 1}
	d := withRunners(f.deps(), map[model.NodeType]nodes.Runner{"request": step.run})
	fl := f.addFlow("once", retriedNode(0, 0))

	events, res := runFlow(testCtx(t), t, d, f.runInput(fl))

	if nf, _ := finishedNode(events, "a"); res.Status != StatusFailed || step.calls != 1 || nf.Tries != 1 {
		t.Fatalf("status %s, calls %d, finished %+v", res.Status, step.calls, nf)
	}
}

func TestRunNode_Given_a_pass_Then_it_is_not_run_again_whatever_the_retries(t *testing.T) {
	f := newFixture(t)
	step := &flaky{}
	d := withRunners(f.deps(), map[model.NodeType]nodes.Runner{"request": step.run})
	fl := f.addFlow("passes", retriedNode(5, 0))

	events, res := runFlow(testCtx(t), t, d, f.runInput(fl))

	if nf, _ := finishedNode(events, "a"); res.Status != StatusPassed || step.calls != 1 || nf.Tries != 1 || warned(events, diag.CodeRunFlaky) {
		t.Fatalf("status %s, calls %d, finished %+v", res.Status, step.calls, nf)
	}
}

func TestRunNode_Given_retries_on_a_step_that_cannot_be_retried_Then_it_runs_once(t *testing.T) {
	f := newFixture(t)
	step := &flaky{failures: 1}
	d := withRunners(f.deps(), map[model.NodeType]nodes.Runner{"assert": step.run})
	fl := f.addFlow("check", model.Node{ID: "a", Type: "assert", Retries: 3})

	_, res := runFlow(testCtx(t), t, d, f.runInput(fl))

	if res.Status != StatusFailed || step.calls != 1 {
		t.Fatalf("a check was run %d times (status %s)", step.calls, res.Status)
	}
}

func TestRunNode_should_wait_retry_wait_before_each_new_try_on_the_runs_clock(t *testing.T) {
	f := newFixture(t)
	step := &flaky{failures: 2}
	var waits []time.Duration
	d := withRunners(f.deps(), map[model.NodeType]nodes.Runner{"request": step.run})
	d.After = func(w time.Duration) <-chan time.Time {
		waits = append(waits, w)
		c := make(chan time.Time, 1)
		c <- time.Time{}
		return c
	}
	fl := f.addFlow("waits", retriedNode(3, 2*time.Second))

	_, res := runFlow(testCtx(t), t, d, f.runInput(fl))

	if res.Status != StatusPassed || len(waits) != 2 || waits[0] != 2*time.Second || waits[1] != 2*time.Second {
		t.Fatalf("status %s, waits %v", res.Status, waits)
	}
}

func TestRunNode_Given_the_run_is_cancelled_while_waiting_Then_it_stops_retrying(t *testing.T) {
	f := newFixture(t)
	step := &flaky{failures: 99}
	d := withRunners(f.deps(), map[model.NodeType]nodes.Runner{"request": step.run})
	ctx, cancel := context.WithCancel(testCtx(t))
	d.After = func(time.Duration) <-chan time.Time {
		cancel() // the person cancels during the pause; the pause never ends by itself
		return make(chan time.Time)
	}
	fl := f.addFlow("cancel", retriedNode(5, time.Hour))

	_, res := runFlow(ctx, t, d, f.runInput(fl))

	if step.calls != 1 || res.Status == StatusPassed {
		t.Fatalf("calls %d, status %s", step.calls, res.Status)
	}
}

func TestRunNode_should_put_the_tries_and_the_flaky_mark_in_the_report(t *testing.T) {
	f := newFixture(t)
	step := &flaky{failures: 1}
	d := withRunners(f.deps(), map[model.NodeType]nodes.Runner{"request": step.run})
	fl := f.addFlow("report", retriedNode(2, 0))

	_, res := runFlow(testCtx(t), t, d, f.runInput(fl))

	n := res.Report.Nodes[0]
	if n.Tries != 2 || !n.Flaky || n.Status != "passed" || !slicesContainsCode(res.Report.Warnings, string(diag.CodeRunFlaky)) {
		t.Fatalf("node %+v warnings %+v", n, res.Report.Warnings)
	}
}

func slicesContainsCode(l []report.Diagnostic, code string) bool {
	for _, d := range l {
		if d.Code == code {
			return true
		}
	}
	return false
}
