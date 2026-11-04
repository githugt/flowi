// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/serve/runs"
)

// watchWorkspace has a flow that reads /health and, when withPost, also posts to /notes. The server's health
// answer is the one in *body.
func watchWorkspace(t *testing.T, body *atomic.Value, withPost bool) (*Workspace, FlowTarget) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/health" {
			_, _ = w.Write([]byte(body.Load().(string)))
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	w := newEnv(t).open(t)
	m, err := w.InitCollection(t.TempDir(), "watched", true)
	if err != nil {
		t.Fatal(err)
	}
	envFile, _ := w.NewEnvironment("local")
	for _, e := range w.Environments() {
		if e.File == envFile {
			e.Vars = []model.EnvVar{{Name: "base_url", Value: srv.URL}}
			if err := w.SaveEnvironment(e, SaveOptions{}); err != nil {
				t.Fatal(err)
			}
		}
	}
	nodes := []model.Node{}
	for name, v := range map[string]model.RequestVersion{"health": {Label: "1", Method: "GET", URL: "{{env.base_url}}/health"}, "notes": {Label: "1", Method: "POST", URL: "{{env.base_url}}/notes"}} {
		if name == "notes" && !withPost {
			continue
		}
		target, err := w.NewRequest(m.ID, name)
		if err != nil {
			t.Fatal(err)
		}
		r := requestOf(t, w, target)
		r.Versions = []model.RequestVersion{v}
		if err := w.SaveRequest(target, r, SaveOptions{}); err != nil {
			t.Fatal(err)
		}
		nodes = append(nodes, model.Node{ID: name, Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: r.ID, Path: name}}})
	}
	flowTarget, err := w.NewFlow(m.ID, "watched")
	if err != nil {
		t.Fatal(err)
	}
	_, flow, _ := w.flowFor(flowTarget)
	flow.Nodes = nodes
	if err := w.SaveFlow(flowTarget, flow, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	return w, flowTarget
}

func TestWatchFlow_should_baseline_the_first_pass_then_report_a_changed_shape(t *testing.T) {
	var body atomic.Value
	body.Store(`{"ok":true,"version":"1"}`)
	w, target := watchWorkspace(t, &body, false)
	opts := RunOptions{Env: "local"}

	first, err := w.WatchFlow(context.Background(), target, opts)
	body.Store(`{"ok":"yes","new":1}`)
	second, err2 := w.WatchFlow(context.Background(), target, opts)

	if err != nil || err2 != nil {
		t.Fatalf("errors %v, %v", err, err2)
	}
	if !first.First || len(first.Change) != 0 || first.Steps != 1 {
		t.Fatalf("first pass %+v", first)
	}
	if second.First || len(second.Change) == 0 || second.Change[0].Kind != "shape" {
		t.Fatalf("second pass %+v: want a shape change", second)
	}
}

func TestWatchFlow_Given_a_step_that_posts_Then_refuses_and_says_which(t *testing.T) {
	var body atomic.Value
	body.Store(`{}`)
	w, target := watchWorkspace(t, &body, true)

	_, err := w.WatchFlow(context.Background(), target, RunOptions{Env: "local"})
	problems, _ := w.WatchProblems(target)

	if err == nil || !strings.Contains(err.Error(), "POST") || len(problems) != 1 {
		t.Fatalf("err %v, problems %v", err, problems)
	}
}

func TestStartRun_Given_a_watch_request_Then_the_outcome_is_changed_when_the_shape_moved(t *testing.T) {
	var body atomic.Value
	body.Store(`{"ok":true,"version":"1"}`)
	w, _ := watchWorkspace(t, &body, false)
	req := runs.Request{Flow: "watched", Env: "local", Watch: true}

	h1, err1 := w.startRun(context.Background(), req)
	baseline := h1.Wait()
	h2, err2 := w.startRun(context.Background(), req)
	unchanged := h2.Wait()
	body.Store(`{"ok":"yes","new":1}`)
	h3, err3 := w.startRun(context.Background(), req)
	moved := h3.Wait()

	if err1 != nil || err2 != nil || err3 != nil {
		t.Fatalf("errors %v, %v, %v", err1, err2, err3)
	}
	if baseline.Status != "passed" || unchanged.Status != "passed" {
		t.Fatalf("baseline %+v, unchanged %+v: want passed", baseline, unchanged)
	}
	if moved.Status != runs.StatusChanged || len(moved.Messages) == 0 || !strings.Contains(moved.Messages[0], "shape") {
		t.Fatalf("moved %+v: want %q with the change in words", moved, runs.StatusChanged)
	}
}

func TestStartRun_Given_a_watch_request_for_a_flow_that_posts_Then_the_start_is_refused(t *testing.T) {
	var body atomic.Value
	body.Store(`{}`)
	w, _ := watchWorkspace(t, &body, true)

	_, err := w.startRun(context.Background(), runs.Request{Flow: "watched", Env: "local", Watch: true})

	var start *runs.StartError
	if !errors.As(err, &start) || start.Kind != runs.KindInvalid || !strings.Contains(start.Message, "POST") {
		t.Fatalf("err %v: want an invalid start that names the POST step", err)
	}
}

func TestWatchHistory_should_list_each_pass_with_what_it_found(t *testing.T) {
	var body atomic.Value
	body.Store(`{"ok":true,"version":"1"}`)
	w, target := watchWorkspace(t, &body, false)
	opts := RunOptions{Env: "local"}
	empty, errEmpty := w.WatchHistory(target)

	_, _ = w.WatchFlow(context.Background(), target, opts)
	body.Store(`{"ok":"yes","new":1}`)
	_, _ = w.WatchFlow(context.Background(), target, opts)
	got, err := w.WatchHistory(target)

	if errEmpty != nil || len(empty) != 0 {
		t.Fatalf("before any pass: %+v, %v", empty, errEmpty)
	}
	if err != nil || len(got) != 2 || got[0].Changes != 0 || got[1].Changes == 0 || got[1].Status != "passed" {
		t.Fatalf("history %+v, err %v", got, err)
	}
}

func TestScheduleWatchHistory_should_find_the_passes_of_the_flow_a_schedule_watches(t *testing.T) {
	var body atomic.Value
	body.Store(`{"ok":true}`)
	w, target := watchWorkspace(t, &body, false)
	_, _ = w.WatchFlow(context.Background(), target, RunOptions{Env: "local"})

	got, err := w.ScheduleWatchHistory(model.Schedule{Name: "c", Flow: "watched", Watch: true})
	none, errNone := w.ScheduleWatchHistory(model.Schedule{Name: "later", Flow: "not-mounted-yet", Watch: true})

	if err != nil || len(got) != 1 {
		t.Fatalf("history %+v, err %v", got, err)
	}
	if errNone != nil || len(none) != 0 {
		t.Fatalf("a flow that cannot be found has no passes, and is no error: %+v, %v", none, errNone)
	}
}
