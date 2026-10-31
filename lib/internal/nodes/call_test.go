// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
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

func fuzzNode(params ...model.Field) model.Node {
	return model.Node{ID: "fuzz-login", Type: "fuzz", Params: params, Request: &model.RequestCall{Request: model.Ref{ID: "01JA0000000000000000000011", Path: "auth/login"}}}
}

func goodBody() map[string]any {
	return map[string]any{"email": "a@x.test", "password": "pw", "age": json.Number("3")}
}

func TestFuzz_Given_a_server_that_rejects_every_bad_body_Then_it_passes_and_counts_the_statuses(t *testing.T) {
	// Arrange
	s, _, rt := flowState()
	rt.fuzz = FuzzSubject{Body: goodBody()}
	rt.requestOut = Outcome{Outputs: map[string]any{"status": 400}, Status: StatusPassed}

	// Act
	out := runOutcome(t, fuzzNode(model.Field{Key: "count", Value: int64(6)}), s)

	// Assert
	if out.Status == StatusFailed || len(out.Diags) != 0 || out.Outputs["cases"] != 6 || out.Outputs["failed"] != 0 {
		t.Fatalf("outcome %+v", out)
	}
	if len(rt.requests) != 6 || rt.requests[0].Body == nil {
		t.Fatalf("each case is sent with its body: %d requests", len(rt.requests))
	}
	if statuses, _ := out.Outputs["statuses"].(map[string]int); statuses["400"] != 6 {
		t.Errorf("statuses %v", out.Outputs["statuses"])
	}
}

func TestFuzz_Given_a_server_error_on_a_bad_body_Then_it_fails_naming_the_case_and_the_status(t *testing.T) {
	// Arrange
	s, _, rt := flowState()
	rt.fuzz = FuzzSubject{Body: goodBody()}
	rt.requestFor = func(c RequestCall) Outcome {
		if body, _ := (*c.Body).(map[string]any); body["email"] == nil {
			return Outcome{Outputs: map[string]any{"status": 500}, Status: StatusPassed}
		}
		return Outcome{Outputs: map[string]any{"status": 422}, Status: StatusPassed}
	}

	// Act
	out := runOutcome(t, fuzzNode(model.Field{Key: "count", Value: int64(40)}), s)

	// Assert
	if out.Status != StatusFailed || len(out.Diags) != 1 || out.Diags[0].Code != diag.CodeRunFuzzFailed {
		t.Fatalf("outcome %+v", out)
	}
	failures, _ := out.Outputs["failures"].([]any)
	if len(failures) == 0 || !strings.Contains(fmt.Sprint(failures[0]), "email: null") || !strings.Contains(fmt.Sprint(failures[0]), "500") {
		t.Fatalf("failures %v", failures)
	}
}

func TestFuzz_Given_expect_rejected_and_a_body_the_server_accepts_Then_it_fails(t *testing.T) {
	// Arrange
	s, _, rt := flowState()
	rt.fuzz = FuzzSubject{Body: goodBody()}
	rt.requestOut = Outcome{Outputs: map[string]any{"status": 200}, Status: StatusPassed}

	// Act
	plain := runOutcome(t, fuzzNode(model.Field{Key: "count", Value: int64(3)}), s)
	strict := runOutcome(t, fuzzNode(model.Field{Key: "count", Value: int64(3)}, model.Field{Key: "expect", Value: "rejected"}), s)

	// Assert
	if plain.Status == StatusFailed {
		t.Errorf("by default only a server error fails: %+v", plain)
	}
	if strict.Status != StatusFailed {
		t.Errorf("expect rejected should fail on a 200: %+v", strict)
	}
}

func TestFuzz_Given_no_cases_param_Then_twenty_are_sent_and_a_body_that_is_not_json_is_a_clear_error(t *testing.T) {
	// Arrange
	s, _, rt := flowState()
	rt.fuzz = FuzzSubject{Body: goodBody()}
	rt.requestOut = Outcome{Outputs: map[string]any{"status": 400}, Status: StatusPassed}

	// Act
	out := runOutcome(t, fuzzNode(), s)
	rt.fuzzErr = errors.New("the request has no JSON body")
	bad := runOutcome(t, fuzzNode(), s)

	// Assert
	if out.Outputs["cases"] != 20 {
		t.Errorf("default cases: %v", out.Outputs["cases"])
	}
	if bad.Status == StatusPassed || len(bad.Diags) == 0 {
		t.Errorf("a request that cannot be fuzzed must say so: %+v", bad)
	}
}

func TestFuzz_Given_a_call_that_got_no_answer_Then_it_is_a_failure(t *testing.T) {
	// Arrange
	s, _, rt := flowState()
	rt.fuzz = FuzzSubject{Body: goodBody()}
	rt.requestOut = Outcome{Status: StatusError}

	// Act
	out := runOutcome(t, fuzzNode(model.Field{Key: "count", Value: int64(2)}), s)

	// Assert
	if out.Status != StatusFailed {
		t.Fatalf("outcome %+v", out)
	}
}
