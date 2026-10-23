// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/nodes"
	"github.com/githugt/flowi-lib/model"
)

func assertDiag() diag.List {
	return diag.List{diag.New(diag.CodeRunAssertFailed, diag.Location{}, map[string]string{"check": "x", "message": "", "node": "n"})}
}

// testCtx bounds a test that would otherwise hang on a scheduling bug.
func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestRunList_should_run_serially_in_list_order_by_default(t *testing.T) {
	f := newFixture(t)
	p := newProbe()
	fl := f.addFlow("serial", probeNode("a"), probeNode("b"), probeNode("c"))

	_, res := runFlow(testCtx(t), t, p.deps(f.deps()), f.runInput(fl))

	want := []string{"start:a", "end:a", "start:b", "end:b", "start:c", "end:c"}
	if res.Status != StatusPassed || !slices.Equal(p.order, want) {
		t.Fatalf("status %s order %v", res.Status, p.order)
	}
}

func TestRunList_Given_needs_without_concurrent_Then_still_serial(t *testing.T) {
	f := newFixture(t)
	p := newProbe()
	fl := f.addFlow("needs", probeNode("a"), probeNode("b"), probeNode("c", "a"))

	runFlow(testCtx(t), t, p.deps(f.deps()), f.runInput(fl))

	if p.starts["c"] < p.ends["b"] {
		t.Fatalf("c started before b ended: %v", p.order)
	}
}

func TestRunList_Given_concurrent_with_needs_Then_overlaps_siblings(t *testing.T) {
	f := newFixture(t)
	p := newProbe()
	cStarted := make(chan struct{})
	p.behave["b"] = func(ctx context.Context, _ *nodes.State) nodes.Outcome {
		select {
		case <-cStarted: // only reachable if c runs while b is running
			return nodes.Outcome{}
		case <-ctx.Done():
			return nodes.Outcome{Status: nodes.StatusCancelled}
		}
	}
	p.behave["c"] = func(context.Context, *nodes.State) nodes.Outcome {
		close(cStarted)
		return nodes.Outcome{}
	}
	fl := f.addFlow("overlap", probeNode("a"), probeNode("b", "a"), concurrentNode("c", "a"), probeNode("d", "b", "c"))

	_, res := runFlow(testCtx(t), t, p.deps(f.deps()), f.runInput(fl))

	if res.Status != StatusPassed || p.starts["d"] < p.ends["b"] || p.starts["d"] < p.ends["c"] {
		t.Fatalf("status %s order %v", res.Status, p.order)
	}
}

func TestRunList_on_error(t *testing.T) {
	tests := []struct {
		name       string
		onError    string
		wantStatus Status
		ran        []string
		skipped    []string
		handled    bool
	}{
		{name: "fail", onError: "", wantStatus: StatusFailed, ran: []string{"a"}, skipped: []string{"b", "cleanup"}},
		{name: "continue", onError: "continue", wantStatus: StatusPassed, ran: []string{"a", "b", "cleanup"}, handled: true},
		{name: "goto", onError: "goto:cleanup", wantStatus: StatusPassed, ran: []string{"a", "cleanup"}, skipped: []string{"b"}, handled: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			p := newProbe()
			p.behave["a"] = failing
			a := probeNode("a")
			a.OnError = tt.onError
			fl := f.addFlow("onerror", a, probeNode("b"), probeNode("cleanup"))

			events, res := runFlow(testCtx(t), t, p.deps(f.deps()), f.runInput(fl))

			if res.Status != tt.wantStatus {
				t.Fatalf("status %s diags %v", res.Status, res.Diags)
			}
			for _, id := range tt.ran {
				if !p.started(id) {
					t.Errorf("%s did not run (%v)", id, p.order)
				}
			}
			for _, id := range tt.skipped {
				if nf, _ := finishedNode(events, id); p.started(id) || nf.Status != StatusSkipped {
					t.Errorf("%s should be skipped: %+v", id, nf)
				}
			}
			a0, _ := finishedNode(events, "a")
			if a0.Status != StatusFailed || a0.Handled != tt.handled || hasCode(res.Diags, diag.CodeRunHandled) != tt.handled {
				t.Fatalf("a %+v diags %v", a0, res.Diags)
			}
		})
	}
}

// branchErrorFlow: start; slow and bad run concurrently after start; bad
// fails once slow is running; after depends on bad; indep needs only slow
// but is serial, so it waits for bad to finish and the failure is always
// seen before indep could start.
func branchErrorFlow(f *fixture, policy string) model.Flow {
	fl := f.addFlow("branch-"+policy, probeNode("start"), concurrentNode("slow", "start"), concurrentNode("bad", "start"),
		probeNode("indep", "slow"), probeNode("after", "bad"))
	fl.Execution.OnBranchError = policy
	return fl
}

func TestRunList_on_branch_error(t *testing.T) {
	tests := []struct {
		policy    string
		slowWaits string // "cancel": until cancelled; "bad": until bad finished
		wantSlow  Status
		wantIndep Status
		wantAfter Status
	}{
		{policy: "fail_fast", slowWaits: "cancel", wantSlow: StatusCancelled, wantIndep: StatusSkipped, wantAfter: StatusSkipped},
		{policy: "finish_running", slowWaits: "bad", wantSlow: StatusPassed, wantIndep: StatusSkipped, wantAfter: StatusSkipped},
		{policy: "continue_all", slowWaits: "bad", wantSlow: StatusPassed, wantIndep: StatusPassed, wantAfter: StatusSkipped},
	}
	for _, tt := range tests {
		t.Run(tt.policy, func(t *testing.T) {
			f := newFixture(t)
			p := newProbe()
			slowStarted, badDone := make(chan struct{}), make(chan struct{})
			p.behave["slow"] = func(ctx context.Context, _ *nodes.State) nodes.Outcome {
				close(slowStarted)
				if tt.slowWaits == "bad" {
					<-badDone
					return nodes.Outcome{}
				}
				<-ctx.Done()
				return nodes.Outcome{Status: nodes.StatusCancelled}
			}
			p.behave["bad"] = func(context.Context, *nodes.State) nodes.Outcome {
				<-slowStarted
				defer close(badDone)
				return failing(context.Background(), nil)
			}
			fl := branchErrorFlow(f, tt.policy)

			events, res := runFlow(testCtx(t), t, p.deps(f.deps()), f.runInput(fl))

			slow, _ := finishedNode(events, "slow")
			indep, _ := finishedNode(events, "indep")
			after, _ := finishedNode(events, "after")
			if res.Status != StatusFailed || slow.Status != tt.wantSlow || indep.Status != tt.wantIndep || after.Status != tt.wantAfter {
				t.Fatalf("run %s slow %s indep %s after %s (%v)", res.Status, slow.Status, indep.Status, after.Status, p.order)
			}
		})
	}
}

// Behaviour change: a node needing a later node waited for it forever; the
// stuck nodes were reported skipped and the run passed, with steps silently
// left out. The run now fails with the dependency cycle.
func TestRunList_Given_a_need_on_a_later_node_Then_the_run_fails_with_the_cycle(t *testing.T) {
	f := newFixture(t)
	p := newProbe()
	fl := f.addFlow("forward", probeNode("a"), probeNode("c", "b"), probeNode("b"))

	_, res := runFlow(testCtx(t), t, p.deps(f.deps()), f.runInput(fl))

	if res.Status == StatusPassed || !slices.Equal(p.order, []string{"start:a", "end:a"}) {
		t.Fatalf("status %s order %v", res.Status, p.order)
	}
}
