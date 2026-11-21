// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
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

// Before: events went out only when another one arrived 33 ms after the last flush, so a NodeStarted that came
// straight after one waited until its step ended: a slow request never showed "running".
func TestFlowRun_should_show_a_step_as_running_while_it_is_still_going(t *testing.T) {
	r, b, w, _ := runFixture(t, nil)
	hold := make(chan struct{})
	b.NextRun = &vmtest.Running{EventList: []engine.Event{engine.NodeStarted{Path: "a", Node: "a"}}, Hold: hold}
	var released atomic.Bool
	showedWhileRunning := false
	// The inline worker runs the whole run inside Run: the run ends when the timer releases it, so a status that
	// arrives before that moment was shown while the step was still going.
	timer := time.AfterFunc(300*time.Millisecond, func() { released.Store(true); close(hold) })
	defer timer.Stop()
	r.OnStatus(func(node, status string) {
		if node == "a" && status == "running" && !released.Load() {
			showedWhileRunning = true
			timer.Stop()
			close(hold)
		}
	})

	r.Run("staging", nil)
	w.Wait()

	if !showedWhileRunning {
		t.Fatal("the step did not show as running until its run ended")
	}
}

// Before: unsaved edits refused the run ("save the flow first"); now the draft is run without being saved.
func TestFlowRun_Given_dirty_doc_Then_runs_the_draft(t *testing.T) {
	r, b, w, _ := runFixture(t, nil)
	r.Doc().Delete("a")

	r.Run("staging", nil)
	w.Wait()

	if r.Err() != "" || !b.Called("RunFlow") || b.LastRunOptions.Draft == nil {
		t.Fatalf("err %q, draft %v", r.Err(), b.LastRunOptions.Draft)
	}
	if len(b.LastRunOptions.Draft.Nodes) != len(r.Doc().Draft().Nodes) {
		t.Fatalf("the run got another flow than the draft")
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

func TestFlowRun_RunPart_should_carry_the_variables_the_last_run_ended_with(t *testing.T) {
	// Arrange
	first := engine.RunResult{Status: engine.StatusPassed}
	first.Report.Vars = map[string]any{"token": "abc"}
	r, b, w, _ := runFixture(t, []engine.Event{engine.RunFinished{Status: engine.StatusPassed}})
	b.NextRun = &vmtest.Running{Result: first}
	r.Run("staging", nil)
	w.Wait()
	b.NextRun = &vmtest.Running{Result: engine.RunResult{Status: engine.StatusPassed}}

	// Act
	ok := r.RunPart("staging", nil, "sub", vm.ScopeOnly)
	w.Wait()

	// Assert
	if !ok || b.LastRunOptions.PriorVars["token"] != "abc" {
		t.Fatalf("ok %v options %+v: the steps that ran must start with the variables the others set", ok, b.LastRunOptions)
	}
}

func TestFlowRun_should_tell_the_listeners_when_a_step_waits_for_the_person_to_sign_in(t *testing.T) {
	// Arrange
	prompt := engine.AuthPrompted{Kind: "device", URL: "https://idp.example/verify", Code: "ABCD"}
	r, _, w, _ := runFixture(t, []engine.Event{prompt, engine.RunFinished{Status: engine.StatusPassed}})
	var got []engine.AuthPrompted
	r.OnPrompt(func(p engine.AuthPrompted) { got = append(got, p) })

	// Act
	r.Run("staging", nil)
	w.Wait()

	// Assert
	if len(got) != 1 || got[0].Code != "ABCD" {
		t.Fatalf("listeners heard %+v", got)
	}
}

func TestEventLine_should_say_what_the_person_is_asked_to_do(t *testing.T) {
	device := vm.EventLine(engine.AuthPrompted{Kind: "device", URL: "https://idp.example/verify", Code: "ABCD"})
	open := vm.EventLine(engine.AuthPrompted{Kind: "open", URL: "https://idp.example/authorize"})

	if !strings.Contains(device, "ABCD") || !strings.Contains(device, "https://idp.example/verify") || !strings.Contains(open, "sign in") {
		t.Fatalf("device %q open %q", device, open)
	}
}

// Before: a step kept only its summed duration, so the run could not be drawn on a timeline.
func TestFlowRun_should_bracket_a_step_with_its_first_start_and_last_finish(t *testing.T) {
	at := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	events := []engine.Event{
		engine.RunStarted{At: at},
		engine.NodeStarted{Path: "users[0]/create", Node: "create", At: at.Add(10 * time.Millisecond)},
		engine.NodeStarted{Path: "users[1]/create", Node: "create", At: at.Add(20 * time.Millisecond)},
		engine.NodeFinished{Path: "users[1]/create", Node: "create", Status: engine.StatusPassed, Duration: 100 * time.Millisecond},
		engine.NodeFinished{Path: "users[0]/create", Node: "create", Status: engine.StatusPassed, Duration: 30 * time.Millisecond},
		engine.RunFinished{Status: engine.StatusPassed},
	}
	r, _, w, _ := runFixture(t, events)

	r.Run("staging", nil)
	w.Wait()

	d, _ := r.Detail("create")
	if !r.Began().Equal(at) || !d.Began.Equal(at.Add(10*time.Millisecond)) || !d.Ended.Equal(at.Add(120*time.Millisecond)) {
		t.Fatalf("run began %v, step %v to %v", r.Began(), d.Began, d.Ended)
	}
}
