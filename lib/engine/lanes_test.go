// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/nodes"
	"github.com/githugt/flowi-lib/model"
)

func batchOf(id string, params []model.Field, bodyNodes ...model.Node) model.Node {
	return model.Node{ID: id, Type: "batch", Params: params, Body: bodyNodes}
}

func TestLanes_should_bound_concurrency_and_keep_result_order(t *testing.T) {
	f := newFixture(t)
	p := newProbe()
	secondDone := make(chan struct{})
	p.behave["work"] = func(ctx context.Context, s *nodes.State) nodes.Outcome {
		if s.Path == "users[0]/work" { // finishes only after iteration 1: out of order
			select {
			case <-secondDone:
			case <-ctx.Done():
			}
		}
		if s.Path == "users[1]/work" {
			defer close(secondDone)
		}
		item, _ := s.Scope().Values["item"].(int)
		s.Vars.Set("mine", item*10)
		return nodes.Outcome{Outputs: map[string]any{"item": item}}
	}
	batch := batchOf("users", []model.Field{{Key: "times", Value: int64(6)}, {Key: "concurrency", Value: int64(2)}}, probeNode("work"))
	fl := f.addFlow("lanes", batch)

	events, res := runFlow(testCtx(t), t, p.deps(f.deps()), f.runInput(fl))

	users, _ := finishedNode(events, "users")
	results, _ := users.Outputs["results"].([]any)
	if res.Status != StatusPassed || len(results) != 6 || p.peak > 2 || p.ends["users[0]/work"] < p.ends["users[1]/work"] {
		t.Fatalf("status %s peak %d results %v order %v", res.Status, p.peak, results, p.order)
	}
	for i, r := range results {
		m := r.(map[string]any)
		vars := m["vars"].(map[string]any)
		work := m["nodes"].(map[string]any)["work"].(map[string]any)
		if m["index"] != i || vars["mine"] != i*10 || work["item"] != i {
			t.Fatalf("result %d: %v", i, m)
		}
	}
}

func parallelOf(id, wait string, branches ...model.Branch) model.Node {
	return model.Node{ID: id, Type: "parallel", Params: []model.Field{{Key: "wait", Value: wait}}, Branches: branches}
}

func TestLanes_wait_any_and_first_success(t *testing.T) {
	tests := []struct {
		wait       string
		quickFails bool
		wantStatus Status
		wantWinner any
		wantCode   diag.Code
	}{
		{wait: "any", wantStatus: StatusPassed, wantWinner: "quick"},
		{wait: "any", quickFails: true, wantStatus: StatusFailed, wantWinner: "quick"},
		{wait: "first-success", quickFails: true, wantStatus: StatusFailed, wantCode: diag.CodeRunNoBranchOK},
	}
	for _, tt := range tests {
		t.Run(tt.wait, func(t *testing.T) {
			f := newFixture(t)
			p := newProbe()
			// Lanes start concurrently. If the quick branch settled the wait before
			// the slow one began, the slow node would be skipped, not cancelled, and
			// the test would depend on scheduling. So quick waits for slow to start.
			slowStarted := make(chan struct{})
			p.behave["q"] = func(context.Context, *nodes.State) nodes.Outcome {
				<-slowStarted
				if tt.quickFails {
					return failing(context.Background(), nil)
				}
				return nodes.Outcome{}
			}
			p.behave["s"] = func(ctx context.Context, _ *nodes.State) nodes.Outcome {
				close(slowStarted)
				if tt.wait == "first-success" && tt.quickFails {
					return failing(context.Background(), nil) // every branch fails
				}
				<-ctx.Done() // only the wait settling can end this branch
				return nodes.Outcome{Status: nodes.StatusCancelled}
			}
			par := parallelOf("both", tt.wait, model.Branch{ID: "slow", Nodes: []model.Node{probeNode("s")}}, model.Branch{ID: "quick", Nodes: []model.Node{probeNode("q")}})
			fl := f.addFlow("wait-"+tt.wait, par)

			events, res := runFlow(testCtx(t), t, p.deps(f.deps()), f.runInput(fl))

			both, _ := finishedNode(events, "both")
			if res.Status != tt.wantStatus || both.Outputs["winner"] != tt.wantWinner {
				t.Fatalf("run %s both %+v", res.Status, both)
			}
			if tt.wantCode != "" && !hasCode(both.Diags, tt.wantCode) {
				t.Fatalf("diags %v", both.Diags)
			}
			if s, _ := finishedNode(events, "both/slow/s"); tt.wait == "any" && s.Status != StatusCancelled {
				t.Fatalf("losing branch %+v", s)
			}
		})
	}
}
