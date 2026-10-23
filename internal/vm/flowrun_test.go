// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"testing"
	"time"

	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
	"github.com/githugt/flowi/internal/vm/vmtest"
)

func runFixture(t *testing.T, events []engine.Event) (*vm.FlowRun, *vmtest.Backend, *vm.Worker, map[string]string) {
	t.Helper()
	b, f := flowFixture()
	f.Nodes = append(f.Nodes, model.Node{ID: "sub", Type: "subflow", Subflow: &model.SubflowCall{Flow: model.Ref{ID: "F9"}}},
		model.Node{ID: "users", Type: "batch", Body: []model.Node{{ID: "create", Type: "log"}}})
	b.Cols[0].Flows = []model.Flow{f}
	b.NextRun = &vmtest.Running{EventList: events, Result: engine.RunResult{Status: engine.StatusFailed}}
	w := vmtest.SyncWorker()
	d := vm.NewFlowDoc(b, w, b.Cols[0], f)
	w.Wait()
	seen := map[string]string{}
	r := vm.NewFlowRun(b, w, d)
	r.OnStatus(func(node, status string) { seen[node] = status })
	return r, b, w, seen
}

func TestFlowRun_maps_events_to_this_flows_nodes(t *testing.T) {
	events := []engine.Event{
		engine.NodeStarted{Path: "a", Node: "a"},
		engine.NodeFinished{Path: "a", Node: "a", Status: engine.StatusPassed, Duration: time.Millisecond, Outputs: map[string]any{"x": "1"}},
		engine.NodeStarted{Path: "p/b1/x", Node: "x"},
		engine.NodeFinished{Path: "p/b1/x", Node: "x", Status: engine.StatusFailed},
		engine.NodeFinished{Path: "x/pre:request:0/a", Node: "a", Status: engine.StatusFailed, Depth: 1}, // a hook step's node
		engine.NodeFinished{Path: "sub/a", Node: "a", Status: engine.StatusError},                        // inside the subflow
		engine.NodeFinished{Path: "users[0]/create", Node: "create", Status: engine.StatusPassed},
		engine.NodeFinished{Path: "users[1]/create", Node: "create", Status: engine.StatusFailed},
		engine.NodeFinished{Path: "users[2]/create", Node: "create", Status: engine.StatusPassed},
		engine.RunFinished{Status: engine.StatusFailed},
	}
	r, b, w, seen := runFixture(t, events)

	r.Run("staging", map[string]string{"email": "a@b.c"})
	w.Wait()

	if seen["a"] != "passed" || seen["x"] != "failed" || seen["create"] != "failed" || r.Running() {
		t.Fatalf("statuses %v running %v", seen, r.Running())
	}
	detail, ok := r.Detail("a")
	if !ok || detail.Status != "passed" || detail.Outputs["x"] != "1" || detail.Instances != 1 {
		t.Fatalf("detail %+v", detail)
	}
	if c, _ := r.Detail("create"); c.Instances != 3 {
		t.Fatalf("create %+v", c)
	}
	if !b.Called("RunFlow") || r.Result().Status != engine.StatusFailed {
		t.Fatalf("calls %v result %+v", b.Calls, r.Result())
	}
}

func TestFlowRun_Given_dirty_doc_Then_refuses(t *testing.T) {
	r, b, w, _ := runFixture(t, nil)
	r.Doc().Delete("a")

	r.Run("staging", nil)
	w.Wait()

	if r.Err() == "" || b.Called("RunFlow") {
		t.Fatalf("err %q", r.Err())
	}
}

func TestFlowRun_RunPart_should_run_the_chosen_steps_and_hand_over_the_others_outputs(t *testing.T) {
	events := []engine.Event{
		engine.NodeFinished{Path: "a", Node: "a", Status: engine.StatusPassed, Outputs: map[string]any{"x": "1"}},
		engine.RunFinished{Status: engine.StatusPassed},
	}
	r, b, w, seen := runFixture(t, events)
	r.Run("staging", nil)
	w.Wait()
	b.NextRun = &vmtest.Running{Result: engine.RunResult{Status: engine.StatusPassed}}

	ok := r.RunPart("staging", nil, "sub", vm.ScopeOnly)
	w.Wait()

	got := b.LastRunOptions
	if !ok || len(got.Only) != 1 || got.Only[0] != "sub" || got.Prior["a"]["x"] != "1" {
		t.Fatalf("ok %v options %+v", ok, got)
	}
	if d, kept := r.Detail("a"); !kept || d.Status != "passed" || seen["a"] != "passed" {
		t.Fatalf("a was cleared: %+v %v", d, seen)
	}
}

func TestFlowRun_RunPart_Given_unknown_step_Then_runs_nothing(t *testing.T) {
	r, b, _, _ := runFixture(t, nil)

	ok := r.RunPart("staging", nil, "zzz", vm.ScopeOnly)

	if ok || b.Called("RunFlow") {
		t.Fatalf("ok %v", ok)
	}
}

func TestFlowRun_should_remember_how_each_hook_of_a_request_step_went(t *testing.T) {
	events := []engine.Event{
		engine.HookFinished{Path: "a", Level: "collection", Phase: "pre", Index: 0, Status: engine.StatusPassed},
		engine.HookFinished{Path: "p/b1/x", Level: "request", Phase: "post", Index: 1, Status: engine.StatusFailed},
		engine.HookFinished{Path: "a", Level: "request", Phase: "post", Index: 0, Status: engine.StatusPassed, Depth: 1}, // inside a hook flow: not this flow's
		engine.HookFinished{Path: "sub", Level: "request", Phase: "pre", Index: 0, Status: engine.StatusPassed},          // a subflow step is not a request of this flow's nodes' hooks
		engine.RunFinished{Status: engine.StatusPassed},
	}
	r, _, w, _ := runFixture(t, events)

	r.Run("staging", nil)
	w.Wait()

	if got := r.HookState("a", vm.HookKey("pre", "collection", 0)); got != "passed" {
		t.Fatalf("a pre: %q", got)
	}
	if got := r.HookState("x", vm.HookKey("post", "request", 1)); got != "failed" {
		t.Fatalf("x post: %q", got)
	}
	if got := r.HookState("a", vm.HookKey("post", "request", 0)); got != "" {
		t.Fatalf("a hook of a hook flow counted: %q", got)
	}
	r.ClearResults()
	if got := r.HookState("a", vm.HookKey("pre", "collection", 0)); got != "" {
		t.Fatalf("after clearing: %q", got)
	}
}
