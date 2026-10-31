// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"fmt"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

func requestNode(id string, r model.Request, with ...model.KeyValue) model.Node {
	return model.Node{ID: id, Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: r.ID, Path: r.Path}, With: with}}
}

func TestRequestNode_headers_should_override_version_headers_but_lose_to_pre_hook_set(t *testing.T) {
	f := newFixture(t)
	v := get("/echo/")
	v.Headers = []model.KeyValue{{Key: "X-A", Value: "version"}, {Key: "X-B", Value: "version"}}
	r := f.addRequest("echo", v)
	f.update(func(c *model.Collection) {
		c.Requests[0].Hooks.Pre = []model.HookStep{inlineHook(setNode("h", "request.headers.X-B", "hook"))}
	})
	n := requestNode("call", r)
	n.Request.Headers = []model.KeyValue{{Key: "X-A", Value: "node {{env.base_url != ''}}"}, {Key: "X-B", Value: "node"}}
	fl := f.addFlow("headers", n)

	events, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	call, _ := finishedNode(events, "call")
	if res.Status != StatusPassed || call.Outputs["status"] != 200 {
		t.Fatalf("result %+v call %+v", res, call)
	}
	headers := f.lastEcho()
	if headers["X-A"] != "node true" || headers["X-B"] != "hook" {
		t.Fatalf("headers %v", headers)
	}
}

func TestRequestNode_should_expose_extract_outputs_status_body_and_time(t *testing.T) {
	f := newFixture(t)
	login := model.RequestVersion{Method: "POST", URL: "{{env.base_url}}/login", Body: model.Body{JSON: `{"password": "{{secret.password}}"}`},
		Extract: []model.Extraction{{Name: "token", From: "body", Path: "$.data.token"}}}
	r := f.addRequest("login", login)
	fl := f.addFlow("outputs", requestNode("login", r), flowNode("check", "assert", "check", "nodes.login.token == 't-1' && nodes.login.status == 200 && nodes.login.body.data.sub == 'u1' && nodes.login.time > 0s"))

	_, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	if res.Status != StatusPassed {
		t.Fatalf("result %+v", res)
	}
}

func TestRequestNode_Given_unknown_request_Then_FLW_REF_001_failed(t *testing.T) {
	f := newFixture(t)
	fl := f.addFlow("unknown", requestNode("call", model.Request{Meta: model.Meta{ID: "01JZ0000000000000000000000"}}))

	events, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	call, _ := finishedNode(events, "call")
	if res.Status != StatusFailed || call.Status != StatusFailed || !hasCode(call.Diags, diag.CodeRefUnknownRequest) {
		t.Fatalf("result %+v call %+v", res, call)
	}
}

func TestRequestNode_Given_connection_refused_Then_status_error(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("down", model.RequestVersion{Method: "GET", URL: "http://127.0.0.1:1/x"})
	fl := f.addFlow("down", requestNode("call", r))

	events, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	call, _ := finishedNode(events, "call")
	if res.Status != StatusError || call.Status != StatusError || !hasCode(call.Diags, diag.CodeHTTPFailed) {
		t.Fatalf("result %+v call %+v", res, call)
	}
}

func TestRun_nested_containers_with_max_concurrency_1_and_hook_requests_Then_completes(t *testing.T) {
	f := newFixture(t)
	cfg := f.addRequest("cfg", get("/config"))
	hookFlow := f.addFlow("hooks/cfg", requestNode("fetch", cfg))
	echo := f.addRequest("echo", get("/echo/{{side}}"))
	f.update(func(c *model.Collection) {
		c.Manifest.Hooks.Pre = []model.HookStep{{Flow: &model.Ref{ID: hookFlow.ID, Path: hookFlow.Path}}}
	})
	par := model.Node{ID: "par", Type: "parallel", Branches: []model.Branch{
		{ID: "l", Nodes: []model.Node{requestNode("left", echo, model.KeyValue{Key: "side", Value: "l"})}},
		{ID: "r", Nodes: []model.Node{requestNode("right", echo, model.KeyValue{Key: "side", Value: "r"})}},
	}}
	batch := batchOf("users", []model.Field{{Key: "times", Value: int64(3)}, {Key: "concurrency", Value: int64(3)}}, par)
	fl := f.addFlow("nested", batch)
	fl.Execution.MaxConcurrency = 1

	events, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	if res.Status != StatusPassed {
		t.Fatalf("result %+v", res)
	}
	for i := range 3 {
		for _, side := range []string{"l/left", "r/right"} {
			if n, _ := finishedNode(events, fmt.Sprintf("users[%d]/par/%s", i, side)); n.Status != StatusPassed {
				t.Fatalf("users[%d]/par/%s: %+v", i, side, n)
			}
		}
	}
	if hits := f.hits.Load(); hits != 12 { // 6 echo requests, each after its hook's /config
		t.Fatalf("hits %d", hits)
	}
}

func TestRun_concurrent_batch_iterations_isolated_and_race_free(t *testing.T) {
	f := newFixture(t)
	echo := f.addRequest("echo", get("/echo/{{n}}"))
	body := []model.Node{
		setNode("mine", "vars.x", "{{index}}"),
		requestNode("call", echo, model.KeyValue{Key: "n", Value: "{{vars.x}}"}),
	}
	batch := batchOf("each", []model.Field{{Key: "times", Value: int64(20)}, {Key: "concurrency", Value: int64(5)}}, body...)
	fl := f.addFlow("isolated", batch)

	events, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	each, _ := finishedNode(events, "each")
	results, _ := each.Outputs["results"].([]any)
	if res.Status != StatusPassed || len(results) != 20 || len(res.Saved) == 0 {
		t.Fatalf("result %+v", res)
	}
	for i, r := range results {
		m := r.(map[string]any)
		if m["vars"].(map[string]any)["x"] != i {
			t.Fatalf("iteration %d vars %v", i, m["vars"])
		}
	}
	if _, leaked := finishedNode(events, "call"); leaked {
		t.Fatal("iteration nodes must be namespaced by iteration")
	}
}

func TestRequestNode_Given_named_outcomes_Then_the_arrow_for_the_one_that_holds_is_taken(t *testing.T) {
	for _, tc := range []struct {
		code                   string
		wantStatus             Status
		notFoundRuns, downRuns bool
	}{
		{"404", StatusPassed, true, false},
		{"503", StatusPassed, false, true},
		{"401", StatusFailed, false, false}, // fails, and no arrow is for an outcome it has
		{"200", StatusPassed, false, false},
	} {
		t.Run(tc.code, func(t *testing.T) {
			f := newFixture(t)
			r := f.addRequest("probe", model.RequestVersion{
				Method: "GET", URL: "{{env.base_url}}/status/" + tc.code, Assert: []string{"response.status < 400"},
				Outcomes: []model.Outcome{
					{Name: "success"}, {Name: "failure"},
					{Name: "not-found", When: "response.status == 404"},
					{Name: "server-down", When: "response.status in [500, 503]"},
				},
			})
			notFound := flowNode("on-404", "note", "text", "x")
			notFound.AfterOutcome = []string{"call:not-found"}
			down := flowNode("on-down", "note", "text", "x")
			down.AfterOutcome = []string{"call:server-down"}
			fl := f.addFlow("outcomes", requestNode("call", r), notFound, down)

			events, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

			ran := func(id string) bool { n, _ := finishedNode(events, id); return n.Status == StatusPassed }
			if res.Status != tc.wantStatus || ran("on-404") != tc.notFoundRuns || ran("on-down") != tc.downRuns {
				t.Fatalf("status %s (want %s); on-404 ran %v (want %v), on-down ran %v (want %v); diags %v",
					res.Status, tc.wantStatus, ran("on-404"), tc.notFoundRuns, ran("on-down"), tc.downRuns, res.Diags)
			}
		})
	}
}

func TestRequestNode_Given_outcomes_on_the_step_Then_they_replace_the_requests_own(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("probe", model.RequestVersion{Method: "GET", URL: "{{env.base_url}}/status/410", Assert: []string{"response.status < 400"},
		Outcomes: []model.Outcome{{Name: "success"}, {Name: "failure"}}}) // the request knows nothing of 410
	gone := flowNode("on-gone", "note", "text", "x")
	gone.AfterOutcome = []string{"call:gone"}
	call := requestNode("call", r)
	call.Request.Outcomes = []model.Outcome{{Name: "success"}, {Name: "failure"}, {Name: "gone", When: "response.status == 410"}}
	fl := f.addFlow("own-outcomes", call, gone)

	events, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	n, _ := finishedNode(events, "on-gone")
	if res.Status != StatusPassed || n.Status != StatusPassed {
		t.Fatalf("run %s, on-gone %s, diags %v", res.Status, n.Status, res.Diags)
	}
}
