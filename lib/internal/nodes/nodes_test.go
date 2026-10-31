// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/httpx"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/template"
	"github.com/githugt/flowi/lib/vault"
)

type recorder struct {
	logs    []string
	warns   diag.List
	digests []string
}

func (r *recorder) Log(node, msg string)   { r.logs = append(r.logs, node+": "+msg) }
func (r *recorder) Warn(d diag.Diagnostic) { r.warns = append(r.warns, d) }
func (r *recorder) Digest(node, format, text string) {
	r.digests = append(r.digests, node+" "+format+": "+text)
}

func varOf(s *State, name string) any {
	v, _ := s.Vars.Get(name)
	return v
}

func node(t model.NodeType, params ...any) model.Node {
	n := model.Node{ID: "n1", Type: t}
	for i := 0; i < len(params); i += 2 {
		n.Params = append(n.Params, model.Field{Key: params[i].(string), Value: params[i+1]})
	}
	return n
}

func newState(phase Phase) (*State, *recorder) {
	rec := &recorder{}
	s := &State{Phase: phase, Vars: NewVars(nil), Emit: rec, Masker: vault.NewMasker("hunter2")}
	if phase == PhasePre {
		s.Request = &MutableRequest{}
	}
	s.Scope = func() template.Scope {
		values := map[string]any{"vars": s.Vars.Snapshot(), "secret": map[string]any{"pw": "hunter2"}}
		if s.Response != nil {
			values["response"] = s.Response.Values()
			values["status"] = s.Response.Status
		}
		return template.Scope{Values: values, Bare: []string{"vars"}}
	}
	return s, rec
}

func run(t *testing.T, n model.Node, s *State) diag.List {
	t.Helper()
	runner, ok := Lookup(n.Type)
	if !ok {
		t.Fatalf("no runner for %s", n.Type)
	}
	return runner(context.Background(), n, s).Diags
}

func sampleResponse() *Response {
	h := http.Header{}
	h.Set("X-Request-Id", "req-42")
	h.Add("Set-Cookie", "sid=abc; Path=/")
	return &Response{
		Status: 201, Header: h, Time: 120 * time.Millisecond,
		Body: map[string]any{"data": map[string]any{"token": "t-1", "items": []any{map[string]any{"id": 1.0}, map[string]any{"id": 2.0}}}},
		Text: `{"data":{"token":"t-1","code":"ABC-123"}}`,
	}
}

func TestSet_should_write_vars_and_request_fields(t *testing.T) {
	tests := []struct {
		target string
		check  func(s *State) bool
	}{
		{"vars.trace", func(s *State) bool { return varOf(s, "trace") == "v" }},
		{"request.method", func(s *State) bool { return *s.Request.Method == "v" }},
		{"request.url", func(s *State) bool { return *s.Request.URL == "v" }},
		{"request.headers.X-Trace", func(s *State) bool { return s.Request.Headers["X-Trace"] == "v" }},
		{"request.query.page", func(s *State) bool { return s.Request.Query["page"] == "v" }},
		{"request.body", func(s *State) bool { return s.Request.Body != nil && *s.Request.Body == "v" }},
		{"request.body.user.name", func(s *State) bool { return s.Request.BodyPaths["user.name"] == "v" }},
		{"cookies.sid", func(s *State) bool { return s.Request.Cookies["sid"] == "v" }},
	}
	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			s, _ := newState(PhasePre)

			diags := run(t, node("set", "target", tt.target, "value", "{{'v'}}"), s)

			if len(diags) != 0 || !tt.check(s) {
				t.Fatalf("diags %v state %+v %+v", diags, s.Vars.Snapshot(), s.Request)
			}
		})
	}
}

func TestSet_should_keep_non_string_values(t *testing.T) {
	s, _ := newState(PhasePre)

	run(t, node("set", "target", "vars.n", "value", "{{1 + 2}}"), s)
	run(t, node("set", "target", "vars.list", "value", []any{"a"}), s)

	if varOf(s, "n") != 3 || !reflect.DeepEqual(varOf(s, "list"), []any{"a"}) {
		t.Fatalf("vars %v", s.Vars.Snapshot())
	}
}

func TestSet_Given_request_target_in_post_phase_Then_FLW_RUN_004(t *testing.T) {
	s, _ := newState(PhasePost)
	s.Response = sampleResponse()

	diags := run(t, node("set", "target", "request.headers.X", "value", "v"), s)

	if len(diags) != 1 || diags[0].Code != diag.CodeRunSetTarget {
		t.Fatalf("diags %v", diags)
	}
}

func TestSet_Given_unknown_target_Then_FLW_RUN_004(t *testing.T) {
	s, _ := newState(PhasePre)

	diags := run(t, node("set", "target", "nowhere.x", "value", "v"), s)

	if len(diags) != 1 || diags[0].Code != diag.CodeRunSetTarget {
		t.Fatalf("diags %v", diags)
	}
}

func TestExtract_from_body_jsonpath_header_cookie_status_and_regex(t *testing.T) {
	jar := []httpx.StoredCookie{{URL: "https://x/", Cookie: http.Cookie{Name: "jarred", Value: "j"}}}
	tests := []struct {
		name string
		x    model.Extraction
		want any
	}{
		{"jsonpath", model.Extraction{From: "body", Path: "$.data.token"}, "t-1"},
		{"jsonpath many", model.Extraction{From: "body", Path: "$.data.items[*].id"}, []any{1.0, 2.0}},
		{"header", model.Extraction{From: "header", Header: "x-request-id"}, "req-42"},
		{"header regex", model.Extraction{From: "header", Header: "X-Request-Id", Regex: `req-(\d+)`}, "42"},
		{"cookie from response", model.Extraction{From: "cookie", Cookie: "sid"}, "abc"},
		{"cookie from jar", model.Extraction{From: "cookie", Cookie: "jarred"}, "j"},
		{"status", model.Extraction{From: "status"}, 201},
		{"body regex", model.Extraction{From: "body", Regex: `"code":"([A-Z]+-\d+)"`}, "ABC-123"},
		{"body regex whole", model.Extraction{From: "body", Regex: `t-\d`}, "t-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Extract(tt.x, sampleResponse(), jar)

			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, %v", got, err)
			}
		})
	}
}

func TestSet_Given_a_response_value_with_no_match_Then_FLW_RUN_005(t *testing.T) {
	s, _ := newState(PhasePost)
	s.Response = sampleResponse()

	diags := run(t, setNode(model.SetValue{Name: "missing", From: "response", Where: "body", Path: "$.nope"}), s)

	if len(diags) != 1 || diags[0].Code != diag.CodeRunExtractEmpty || diags[0].Args["name"] != "missing" {
		t.Fatalf("diags %v", diags)
	}
}

func TestSet_Given_values_in_a_post_hook_Then_they_read_the_response(t *testing.T) {
	s, _ := newState(PhasePost)
	s.Response = sampleResponse()

	diags := run(t, setNode(
		model.SetValue{Name: "token", From: "response", Where: "body", Path: "$.data.token"},
		model.SetValue{Name: "copy", Value: "{{response.body.data.token}}"},
		model.SetValue{Name: "code", Value: "{{status}}"},
	), s)

	if len(diags) != 0 || varOf(s, "token") != "t-1" || varOf(s, "copy") != "t-1" || varOf(s, "code") != 201 {
		t.Fatalf("diags %v vars %v", diags, s.Vars.Snapshot())
	}
}

func TestAssert_Given_true_Then_no_diagnostics(t *testing.T) {
	s, _ := newState(PhasePost)
	s.Response = sampleResponse()

	diags := run(t, node("assert", "check", "status == 201 && response.time < 1s"), s)

	if len(diags) != 0 {
		t.Fatalf("diags %v", diags)
	}
}

func TestAssert_Given_false_and_on_fail_warn_Then_warning_only(t *testing.T) {
	s, rec := newState(PhasePost)
	s.Response = sampleResponse()

	diags := run(t, node("assert", "check", "response.time < 100ms", "on_fail", "warn", "message", "slow"), s)

	if len(diags) != 0 || len(rec.warns) != 1 || rec.warns[0].Code != diag.CodeRunAssertWarning || !strings.Contains(rec.warns[0].Args["message"], "slow") {
		t.Fatalf("diags %v warns %v", diags, rec.warns)
	}
}

func TestAssert_Given_false_Then_FLW_RUN_001_with_check_text(t *testing.T) {
	s, _ := newState(PhasePost)
	s.Response = sampleResponse()

	diags := run(t, node("assert", "check", "status == 200"), s)

	if len(diags) != 1 || diags[0].Code != diag.CodeRunAssertFailed || diags[0].Args["check"] != "status == 200" {
		t.Fatalf("diags %v", diags)
	}
}

func TestAssert_Given_undefined_reference_Then_FLW_TPL_001(t *testing.T) {
	s, _ := newState(PhasePost)
	s.Response = sampleResponse()

	diags := run(t, node("assert", "check", "vars.nope == 1"), s)

	if len(diags) != 1 || diags[0].Code != diag.CodeTemplateUndefined {
		t.Fatalf("diags %v", diags)
	}
}

// A message built from a secret is masked whole. Masking only the secret's
// substring (the earlier behaviour) leaked values derived from it, such as
// base64("user:"+secret), which contain no substring of the secret.
func TestLog_Given_message_built_from_secret_Then_whole_message_masked(t *testing.T) {
	s, rec := newState(PhasePre)

	diags := run(t, node("log", "value", "pw={{secret.pw}}"), s)

	if len(diags) != 0 || len(rec.logs) != 1 || rec.logs[0] != "n1: "+vault.Mask {
		t.Fatalf("diags %v logs %v", diags, rec.logs)
	}
}

func TestLookup_should_know_registered_types_only(t *testing.T) {
	for _, nt := range []model.NodeType{"set", "assert", "log", "note"} {
		if _, ok := Lookup(nt); !ok {
			t.Errorf("%s missing", nt)
		}
	}
	if _, ok := Lookup("nope"); ok {
		t.Error("unknown type should not resolve")
	}
}

func TestLog_Given_plain_message_Then_known_secrets_still_masked(t *testing.T) {
	s, rec := newState(PhasePre)
	s.Vars.Set("copy", "hunter2")

	run(t, node("log", "value", "copy={{vars.copy}}"), s)

	if len(rec.logs) != 1 || rec.logs[0] != "n1: copy="+vault.Mask {
		t.Fatalf("logs %v", rec.logs)
	}
}

func TestSet_Given_value_built_from_secret_Then_registered_as_secret(t *testing.T) {
	s, _ := newState(PhasePre)
	var got []any
	s.Secret = func(v any) { got = append(got, v) }

	run(t, node("set", "target", "vars.basic", "value", `{{toBase64("user:" + secret.pw)}}`), s)
	run(t, node("set", "target", "vars.plain", "value", "x"), s)

	if len(got) != 1 || got[0] != varOf(s, "basic") {
		t.Fatalf("registered %v", got)
	}
}

func TestExtract_Given_an_xml_body_and_a_path_that_starts_with_a_slash_Then_it_is_read_as_an_xml_path(t *testing.T) {
	// Arrange
	resp := &Response{Status: 207, Header: http.Header{"Content-Type": {"application/xml"}}, Text: `<D:multistatus xmlns:D="DAV:"><D:response><D:href>/a</D:href></D:response><D:response><D:href>/b</D:href></D:response></D:multistatus>`}
	tests := []struct {
		name, path string
		want       any
	}{
		{"one match is the value", "/multistatus/response[2]/href", "/b"},
		{"several are a list", "//href", []any{"/a", "/b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := Extract(model.Extraction{From: "body", Path: tt.path}, resp, nil)

			// Assert
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, %v", got, err)
			}
		})
	}
}

func TestExtract_Given_an_xml_path_that_matches_nothing_or_is_wrong_Then_an_error_says_so(t *testing.T) {
	resp := &Response{Status: 200, Text: `<a><b>1</b></a>`}

	_, nothing := Extract(model.Extraction{From: "body", Path: "/a/c"}, resp, nil)
	_, bad := Extract(model.Extraction{From: "body", Path: "//a[0]"}, resp, nil)

	if !errors.Is(nothing, errNothing) || bad == nil || !strings.Contains(bad.Error(), "from 1") {
		t.Fatalf("nothing %v bad %v", nothing, bad)
	}
}
