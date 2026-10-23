// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

func TestRequest_should_render_with_and_headers_and_return_runtime_outcome(t *testing.T) {
	s, _, rt := flowState()
	s.Vars.Set("token", "t-1")
	s.Vars.Set("n", 3)
	rt.requestOut = Outcome{Outputs: map[string]any{"status": 200}, Status: StatusPassed}
	n := model.Node{ID: "verify", Type: "request", Request: &model.RequestCall{
		Request: model.Ref{ID: "01JA0000000000000000000011", Path: "auth/verify"}, Version: "1",
		Hooks:   model.HookControl{Skip: []string{"post"}},
		With:    []model.KeyValue{{Key: "token", Value: "{{vars.token}}"}, {Key: "count", Value: "{{vars.n}}"}},
		Headers: []model.KeyValue{{Key: "Authorization", Value: "Bearer {{vars.token}}"}},
	}}

	out := runOutcome(t, n, s)

	if out.Status != StatusPassed || out.Outputs["status"] != 200 || len(rt.requests) != 1 {
		t.Fatalf("outcome %+v", out)
	}
	c := rt.requests[0]
	if c.Ref.ID != "01JA0000000000000000000011" || c.Version != "1" || c.Hooks.Skip[0] != "post" ||
		c.With["token"] != "t-1" || c.With["count"] != 3 || c.Headers["Authorization"] != "Bearer t-1" {
		t.Fatalf("call %+v", c)
	}
}

func TestRequest_Given_undefined_with_Then_template_error_and_no_call(t *testing.T) {
	s, _, rt := flowState()
	n := model.Node{ID: "r", Type: "request", Request: &model.RequestCall{With: []model.KeyValue{{Key: "x", Value: "{{vars.missing}}"}}}}

	out := runOutcome(t, n, s)

	if len(out.Diags) != 1 || out.Diags[0].Code != diag.CodeTemplateUndefined || len(rt.requests) != 0 {
		t.Fatalf("outcome %+v", out)
	}
}

func TestSubflow_should_pass_rendered_inputs(t *testing.T) {
	s, _, rt := flowState()
	s.Vars.Set("email", "a@x")
	rt.subflowOut = Outcome{Outputs: map[string]any{"token": "t"}}
	n := model.Node{ID: "login", Type: "subflow", Subflow: &model.SubflowCall{
		Flow: model.Ref{ID: "01JA0000000000000000000030"}, With: []model.KeyValue{{Key: "email", Value: "{{vars.email}}"}}}}

	out := runOutcome(t, n, s)

	if out.Outputs["token"] != "t" || len(rt.subflows) != 1 || rt.subflows[0]["email"] != "a@x" {
		t.Fatalf("outcome %+v subflows %v", out, rt.subflows)
	}
}

func okResponse() HTTPResult {
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	return HTTPResult{Status: StatusPassed, Response: &Response{
		Status: 200, Header: h, Body: map[string]any{"items": []any{map[string]any{"id": 1}, map[string]any{"id": 2}}}, Time: 40 * time.Millisecond}}
}

func TestFetch_should_build_call_and_output_transform_value(t *testing.T) {
	s, _, rt := flowState()
	s.Vars.Set("q", "ada")
	rt.httpOut = okResponse()
	n := node("fetch", "url", "http://api.test/users", "method", "post",
		"headers", []model.Field{{Key: "X-Q", Value: "{{vars.q}}"}},
		"query", []model.Field{{Key: "q", Value: "{{vars.q}}"}},
		"body", []model.Field{{Key: "name", Value: "{{vars.q}}"}, {Key: "n", Value: int64(1)}},
		"transform", "{{map(response.body.items, .id)}}")

	out := runOutcome(t, n, s)

	if len(out.Diags) != 0 || out.Outputs["status"] != 200 || fmt.Sprint(out.Outputs["value"]) != "[1 2]" ||
		out.Outputs["time"] != 40*time.Millisecond {
		t.Fatalf("outcome %+v", out)
	}
	headers, _ := out.Outputs["headers"].(map[string]any)
	if headers["content-type"] != "application/json" {
		t.Fatalf("headers %v", out.Outputs["headers"])
	}
	c := rt.https[0]
	body, _ := json.Marshal(c.Body)
	if c.Method != "POST" || c.URL != "http://api.test/users" || c.Headers["X-Q"] != "ada" || c.Query["q"] != "ada" || string(body) != `{"n":1,"name":"ada"}` {
		t.Fatalf("call %+v body %s", c, body)
	}
}

func TestFetch_Given_string_body_and_no_method_Then_GET_with_text(t *testing.T) {
	s, _, rt := flowState()
	rt.httpOut = okResponse()

	runOutcome(t, node("fetch", "url", "http://api.test/x", "body", "hello {{'ada'}}"), s)

	if c := rt.https[0]; c.Method != "GET" || c.Body != "hello ada" {
		t.Fatalf("call %+v", c)
	}
}

func TestFetch_Given_transport_failure_Then_runtime_status_and_diags(t *testing.T) {
	s, _, rt := flowState()
	rt.httpOut = HTTPResult{Status: StatusError, Diags: diag.List{diag.New(diag.CodeHTTPFailed, diag.Location{}, map[string]string{"detail": "refused"})}}

	out := runOutcome(t, node("fetch", "url", "http://api.test/x"), s)

	if out.Status != StatusError || len(out.Diags) != 1 || out.Outputs != nil {
		t.Fatalf("outcome %+v", out)
	}
}

func TestWebhook_should_post_payload_and_wait_by_default(t *testing.T) {
	s, _, rt := flowState()
	rt.httpOut = okResponse()

	out := runOutcome(t, node("webhook", "url", "http://hooks.test/x", "payload", []model.Field{{Key: "ok", Value: true}}), s)

	if out.Outputs["status"] != 200 || len(rt.https) != 1 || rt.https[0].Method != "POST" || len(rt.background) != 0 {
		t.Fatalf("outcome %+v", out)
	}
}

func TestWebhook_Given_wait_false_Then_background_and_no_outputs(t *testing.T) {
	s, _, rt := flowState()

	out := runOutcome(t, node("webhook", "url", "http://hooks.test/x", "wait", false), s)

	if out.Outputs != nil || len(rt.background) != 1 || len(rt.https) != 0 || rt.background[0].Method != "POST" {
		t.Fatalf("outcome %+v background %v", out, rt.background)
	}
}
