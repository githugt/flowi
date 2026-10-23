// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/template"
	"github.com/githugt/flowi-lib/vault"
)

func testScope(values map[string]any) template.Scope {
	return template.Scope{Values: values, Bare: []string{"with", "vars", "inputs", "env"}}
}

func baseInput(v model.RequestVersion, values map[string]any) buildInput {
	return buildInput{version: v, scope: testScope(values), empty: template.EmptyOmit, label: "auth/login", root: "."}
}

func bodyText(t *testing.T, b built) string {
	t.Helper()
	if b.request.Body == nil {
		return ""
	}
	r, _, _, err := b.request.Body.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	data, _ := io.ReadAll(r)
	return string(data)
}

func TestBuild_should_render_url_query_headers_and_json_body(t *testing.T) {
	v := model.RequestVersion{
		Method:  "POST",
		URL:     "{{env.base_url}}/v1/login",
		Query:   []model.KeyValue{{Key: "lang", Value: "en gb"}, {Key: "trace", Value: "{{vars.trace}}"}},
		Headers: []model.KeyValue{{Key: "X-Trace", Value: "{{vars.trace}}"}},
		Body:    model.Body{JSON: `{ "email": "{{email}}" }`},
	}
	values := map[string]any{
		"env":  map[string]any{"base_url": "https://api.example"},
		"vars": map[string]any{"trace": "t-1"},
		"with": map[string]any{"email": `a"b@example.com`},
	}

	b, diags := build(baseInput(v, values))

	if len(diags) != 0 {
		t.Fatalf("diags %v", diags)
	}
	if b.request.Method != "POST" || b.request.URL != "https://api.example/v1/login?lang=en+gb&trace=t-1" {
		t.Fatalf("request %s %s", b.request.Method, b.request.URL)
	}
	if b.request.Header.Get("X-Trace") != "t-1" || b.request.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("headers %v", b.request.Header)
	}
	if got := bodyText(t, b); got != `{ "email": "a\"b@example.com" }` {
		t.Fatalf("body %s", got)
	}
	if b.sent.Body == nil || b.sent.Body.Text != `{ "email": "a\"b@example.com" }` || b.sent.Header[0] != (Header{Name: "X-Trace", Value: "t-1"}) {
		t.Fatalf("sent %+v", b.sent)
	}
}

func TestBuild_Given_empty_header_value_Then_follows_policy(t *testing.T) {
	v := model.RequestVersion{Method: "GET", URL: "http://x", Headers: []model.KeyValue{{Key: "X-Opt", Value: "{{vars.blank}}"}}}
	values := map[string]any{"vars": map[string]any{"blank": ""}}
	tests := []struct {
		policy  template.EmptyPolicy
		present bool
		code    diag.Code
	}{
		{template.EmptyKeep, true, ""},
		{template.EmptyOmit, false, ""},
		{template.EmptyError, false, diag.CodeTemplateEmpty},
	}
	for _, tt := range tests {
		in := baseInput(v, values)
		in.empty = tt.policy

		b, diags := build(in)

		_, present := b.request.Header["X-Opt"]
		if present != tt.present {
			t.Errorf("policy %v: present %v", tt.policy, present)
		}
		if tt.code != "" && (len(diags) != 1 || diags[0].Code != tt.code || !strings.Contains(diags[0].Args["field"], "X-Opt")) {
			t.Errorf("policy %v: diags %v", tt.policy, diags)
		}
	}
}

func TestBuild_Given_undefined_variable_Then_FLW_TPL_001_names_variable_and_field(t *testing.T) {
	v := model.RequestVersion{Method: "GET", URL: "http://x/{{vars.user_id}}"}

	_, diags := build(baseInput(v, map[string]any{"vars": map[string]any{}}))

	if len(diags) != 1 || diags[0].Code != diag.CodeTemplateUndefined || diags[0].Args["name"] != "vars.user_id" {
		t.Fatalf("diags %v", diags)
	}
	msg, _ := diag.English().Message(diags[0])
	if !strings.Contains(msg, "vars.user_id") || !strings.Contains(msg, "auth/login") {
		t.Fatalf("message %q does not name the variable and the request", msg)
	}
}

func TestBuild_Given_env_ref_to_secret_var_Then_hint_suggests_secret_scope(t *testing.T) {
	v := model.RequestVersion{Method: "GET", URL: "http://x", Headers: []model.KeyValue{{Key: "X-Key", Value: "{{env.api_key}}"}}}
	in := baseInput(v, map[string]any{"env": map[string]any{}})
	in.secretVars = map[string]bool{"api_key": true}

	_, diags := build(in)

	if len(diags) != 1 || !strings.Contains(diags[0].Args["hint"], "secret.api_key") {
		t.Fatalf("diags %v", diags)
	}
}

func TestBuild_Given_missing_declared_secret_Then_left_to_caller(t *testing.T) {
	v := model.RequestVersion{Method: "GET", URL: "http://x", Headers: []model.KeyValue{{Key: "X-Key", Value: "{{secret.api_key}}"}}}
	in := baseInput(v, map[string]any{"secret": map[string]any{}})
	in.missingSecret = func(name string) bool { return name == "api_key" }

	_, diags := build(in)

	if len(diags) != 0 {
		t.Fatalf("diags %v", diags)
	}
}

func TestBuild_Given_header_with_newline_Then_FLW_REQ_005(t *testing.T) {
	v := model.RequestVersion{Method: "GET", URL: "http://x", Headers: []model.KeyValue{{Key: "X-Bad", Value: "{{vars.v}}"}}}

	_, diags := build(baseInput(v, map[string]any{"vars": map[string]any{"v": "a\r\nInjected: yes"}}))

	if len(diags) != 1 || diags[0].Code != diag.CodeRequestHeader {
		t.Fatalf("diags %v", diags)
	}
}

func TestBuild_Given_file_body_outside_root_Then_FLW_REQ_003(t *testing.T) {
	in := baseInput(model.RequestVersion{Method: "POST", URL: "http://x", Body: model.Body{File: "../../etc/passwd"}}, nil)
	in.root = t.TempDir()

	_, diags := build(in)

	if len(diags) != 1 || diags[0].Code != diag.CodeRequestFileOutside {
		t.Fatalf("diags %v", diags)
	}
}

func TestBuild_Given_missing_file_body_Then_FLW_REQ_006(t *testing.T) {
	in := baseInput(model.RequestVersion{Method: "POST", URL: "http://x", Body: model.Body{File: "fixtures/none.pdf"}}, nil)
	in.root = t.TempDir()

	_, diags := build(in)

	if len(diags) != 1 || diags[0].Code != diag.CodeRequestFileRead {
		t.Fatalf("diags %v", diags)
	}
}

func TestBuild_should_send_form_and_multipart_bodies(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "doc.txt"), []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	form := baseInput(model.RequestVersion{Method: "POST", URL: "http://x", Body: model.Body{Form: []model.KeyValue{{Key: "a", Value: "{{vars.v}}"}}}}, map[string]any{"vars": map[string]any{"v": "1 2"}})
	multi := baseInput(model.RequestVersion{Method: "POST", URL: "http://x", Body: model.Body{Multipart: []model.Field{
		{Key: "title", Value: "{{vars.v}}"},
		{Key: "doc", Value: map[string]any{"file": "doc.txt", "content_type": "text/plain"}},
	}}}, map[string]any{"vars": map[string]any{"v": "T"}})
	multi.root = root

	fb, fd := build(form)
	mb, md := build(multi)

	if len(fd) != 0 || bodyText(t, fb) != "a=1+2" {
		t.Fatalf("form %v %q", fd, bodyText(t, fb))
	}
	if text := bodyText(t, mb); len(md) != 0 || !strings.Contains(text, `name="title"`) || !strings.Contains(text, "hi") {
		t.Fatalf("multipart %v %q", md, text)
	}
}

func TestBuild_Given_basic_auth_from_secret_Then_header_marked_tainted(t *testing.T) {
	v := model.RequestVersion{Method: "GET", URL: "http://x?k={{secret.key}}&q=1", Headers: []model.KeyValue{
		{Key: "Authorization", Value: `Basic {{toBase64("me:" + secret.pw)}}`},
		{Key: "X-Plain", Value: "plain"},
	}}
	values := map[string]any{"secret": map[string]any{"pw": "hunter2", "key": "k1"}}

	b, diags := build(baseInput(v, values))

	if len(diags) != 0 || !b.tainted["header:Authorization"] || b.tainted["header:X-Plain"] || !b.tainted["url"] {
		t.Fatalf("diags %v tainted %v", diags, b.tainted)
	}
	if b.sent.Header[0].Value != vault.Mask || b.sent.URL != vault.Mask || b.sent.Header[1].Value != "plain" {
		t.Fatalf("sent %+v", b.sent)
	}
	if b.request.Header.Get("Authorization") == vault.Mask {
		t.Fatal("real request masked")
	}
}

func TestBuild_should_add_default_headers_the_request_does_not_set(t *testing.T) {
	v := model.RequestVersion{Method: "GET", URL: "http://x.test/", Headers: []model.KeyValue{{Key: "accept", Value: "text/plain"}}}
	in := baseInput(v, map[string]any{})
	in.defaults = []model.KeyValue{{Key: "Accept", Value: "*/*"}, {Key: "Cache-Control", Value: "no-cache"}, {Key: "User-Agent", Value: "Flowi-UI/1"}}

	b, diags := build(in)

	if len(diags) != 0 {
		t.Fatalf("diags %v", diags)
	}
	if got := b.request.Header.Values("Accept"); len(got) != 1 || got[0] != "text/plain" {
		t.Fatalf("Accept %v: the request's own header must win", got)
	}
	if b.request.Header.Get("Cache-Control") != "no-cache" || b.request.Header.Get("User-Agent") != "Flowi-UI/1" {
		t.Fatalf("headers %v", b.request.Header)
	}
	if len(b.sent.Header) != 3 {
		t.Fatalf("recorded headers %v", b.sent.Header)
	}
}

// Setting a header empty is how a request opts out of its default.
func TestBuild_Given_an_empty_header_Then_the_default_is_not_added(t *testing.T) {
	v := model.RequestVersion{Method: "GET", URL: "http://x.test/", Headers: []model.KeyValue{{Key: "Cache-Control", Value: ""}}}
	in := baseInput(v, map[string]any{})
	in.defaults = []model.KeyValue{{Key: "Cache-Control", Value: "no-cache"}}

	b, _ := build(in)

	if b.request.Header.Get("Cache-Control") != "" {
		t.Fatalf("headers %v", b.request.Header)
	}
}
