// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"testing"
	"time"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
)

func logNode(id string) model.Node {
	return model.Node{ID: id, Type: "log", Params: []model.Field{{Key: "value", Value: "x"}}}
}

// pipelineOf returns the rows of a request with two pre nodes and two post
// nodes (inline), in one hook step each.
func pipelineOf(t *testing.T, trace *vm.Trace) []vm.PipelineRow {
	t.Helper()
	r, _ := newRequestVM(t, sampleBackend())
	r.Edit(func(v *model.RequestVersion) {
		v.Hooks.Pre = []model.HookStep{{Inline: []model.Node{logNode("a"), logNode("b")}}}
		v.Hooks.Post = []model.HookStep{{Inline: []model.Node{logNode("c"), logNode("d")}}}
	})
	return r.Pipeline(trace)
}

func titles(rows []vm.PipelineRow) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r.Title)
	}
	return out
}

func TestPipeline_should_list_pre_hooks_then_the_request_then_post_hooks(t *testing.T) {
	rows := pipelineOf(t, nil)

	var kinds []vm.RowKind
	for _, r := range rows {
		kinds = append(kinds, r.Kind)
	}
	want := []vm.RowKind{vm.RowSection, vm.RowNode, vm.RowNode, vm.RowAPI, vm.RowSection, vm.RowNode, vm.RowNode}
	if len(kinds) != len(want) {
		t.Fatalf("rows %v", titles(rows))
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("row %d is kind %v, want %v (%v)", i, kinds[i], want[i], titles(rows))
		}
	}
	if rows[0].Title != "Pre-hooks" || rows[1].Title != "a" || rows[4].Title != "Post-hooks" || rows[6].Title != "d" {
		t.Fatalf("titles %v", titles(rows))
	}
	api := rows[3]
	if api.Method == "" || api.Tip == "" {
		t.Fatalf("api row %+v", api)
	}
	for _, r := range rows {
		if r.State != vm.StepIdle {
			t.Fatalf("%q is %v before any send", r.Title, r.State)
		}
	}
}

func TestFlowTone_should_say_how_the_last_send_went(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())
	run := func(events ...engine.Event) *vm.Trace {
		trace := vm.NewTrace()
		for _, e := range events {
			trace.Apply(e)
		}
		return trace
	}
	warn := engine.Warning{Diag: diag.New(diag.CodeRunAssertWarning, diag.Location{}, map[string]string{"check": "x", "message": "", "node": "n"})}
	sent := engine.RequestSent{Method: "GET", URL: "https://x.test"}
	ok := engine.ResponseReceived{Status: 200}
	finished := func(t *vm.Trace) *vm.Trace { t.Finish(); return t }

	cases := []struct {
		name  string
		trace *vm.Trace
		want  vm.Tone
	}{
		{"no send yet", nil, vm.ToneNone},
		{"still running", run(sent), vm.TonePending},
		{"passed", finished(run(sent, ok)), vm.ToneSuccess},
		{"passed with a warning", finished(run(sent, ok, warn)), vm.ToneClientError},
		{"a hook failed", finished(run(sent, ok, engine.NodeFinished{Lane: "post:version:0", Node: "c", Status: engine.StatusFailed, Depth: 1})), vm.ToneServerError},
		{"no connection", finished(run(sent, engine.ResponseReceived{Error: "no such host"})), vm.ToneServerError},
		{"a failure and a warning", finished(run(sent, ok, warn, engine.NodeFinished{Lane: "pre:version:0", Node: "a", Status: engine.StatusFailed, Depth: 1})), vm.ToneServerError},
	}
	for _, c := range cases {
		if got := r.FlowTone(c.trace); got != c.want {
			t.Errorf("%s: tone %v, want %v", c.name, got, c.want)
		}
	}
}

func TestPipeline_Given_a_request_without_hooks_Then_only_the_request_is_shown(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())

	rows := r.Pipeline(nil)

	if len(rows) != 1 || rows[0].Kind != vm.RowAPI {
		t.Fatalf("rows %v", titles(rows))
	}
}

func TestPipeline_Given_a_send_that_failed_in_a_post_hook_Then_marks_passed_failed_and_not_run(t *testing.T) {
	trace := vm.NewTrace()
	pre, post := "pre:version:0", "post:version:0"
	for _, e := range []engine.Event{
		engine.NodeFinished{Lane: pre, Node: "a", Status: engine.StatusPassed, Depth: 1},
		engine.NodeFinished{Lane: pre, Node: "b", Status: engine.StatusPassed, Depth: 1},
		engine.RequestSent{Method: "GET", URL: "https://x.test/real"},
		engine.ResponseReceived{Status: 404},
		engine.NodeFinished{Lane: post, Node: "c", Status: engine.StatusFailed, Depth: 1,
			Diags: diag.List{diag.New(diag.CodeRunAssertFailed, diag.Location{}, map[string]string{"check": "x", "message": " — Expected a 2xx response, got 404", "node": "c"})}},
	} {
		trace.Apply(e)
	}
	trace.Finish()

	rows := pipelineOf(t, trace)

	got := map[string]vm.PipelineRow{}
	for _, r := range rows {
		got[r.Title] = r
	}
	if got["a"].State != vm.StepPassed || got["b"].State != vm.StepPassed {
		t.Fatalf("pre nodes %v %v", got["a"].State, got["b"].State)
	}
	if api := rows[3]; api.State != vm.StepPassed || api.Tip != "https://x.test/real" || api.Detail != "404" {
		t.Fatalf("api %+v", api)
	}
	if c := got["c"]; c.State != vm.StepFailed || c.Reason == "" {
		t.Fatalf("c %+v", c)
	}
	if got["d"].State != vm.StepNotRun {
		t.Fatalf("d is %v, want not run", got["d"].State)
	}
}

func TestPipeline_Given_a_pre_hook_failed_Then_the_request_and_post_hooks_are_not_run(t *testing.T) {
	trace := vm.NewTrace()
	trace.Apply(engine.NodeFinished{Lane: "pre:version:0", Node: "a", Status: engine.StatusFailed, Depth: 1})
	trace.Finish()

	rows := pipelineOf(t, trace)

	if rows[3].State != vm.StepNotRun || rows[5].State != vm.StepNotRun {
		t.Fatalf("api %v, post %v", rows[3].State, rows[5].State)
	}
}

func TestPipeline_Given_a_failed_request_Then_the_error_is_the_reason(t *testing.T) {
	trace := vm.NewTrace()
	trace.Apply(engine.RequestSent{Method: "GET", URL: "https://x.test"})
	trace.Apply(engine.ResponseReceived{Error: "dial tcp: no such host"})
	trace.Finish()

	rows := pipelineOf(t, trace)

	if api := rows[3]; api.State != vm.StepFailed || api.Reason != "dial tcp: no such host" {
		t.Fatalf("api %+v", api)
	}
}

func TestPipeline_Given_a_send_in_progress_Then_unfinished_steps_are_idle_not_not_run(t *testing.T) {
	trace := vm.NewTrace()
	trace.Apply(engine.NodeFinished{Lane: "pre:version:0", Node: "a", Status: engine.StatusPassed, Depth: 1})

	rows := pipelineOf(t, trace)

	if rows[2].State != vm.StepIdle || rows[3].State != vm.StepIdle {
		t.Fatalf("b %v, api %v", rows[2].State, rows[3].State)
	}
}

func TestPipeline_should_carry_how_long_each_finished_step_took(t *testing.T) {
	trace := vm.NewTrace()
	trace.Apply(engine.NodeFinished{Lane: "pre:version:0", Node: "a", Status: engine.StatusPassed, Depth: 1, Duration: 12 * time.Millisecond})
	trace.Apply(engine.RequestSent{Method: "GET", URL: "https://x.test"})
	trace.Apply(engine.ResponseReceived{Status: 200, Duration: 322 * time.Millisecond})
	trace.Finish()

	rows := pipelineOf(t, trace)

	var a, api vm.PipelineRow
	for _, r := range rows {
		switch r.Key {
		case "api":
			api = r
		case "pre:version:0/a":
			a = r
		}
	}
	if a.Duration != 12*time.Millisecond || api.Duration != 322*time.Millisecond {
		t.Fatalf("hook %v request %v (rows %v)", a.Duration, api.Duration, titles(rows))
	}
}
