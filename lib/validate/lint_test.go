// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

func TestLintValue(t *testing.T) {
	tests := []struct {
		value    string
		wantCode diag.Code
		wantChar string
	}{
		{" x", diag.CodeLintWhitespace, ""},
		{"x ", diag.CodeLintWhitespace, ""},
		{"a\tb", diag.CodeLintControl, ""},
		{"a\nb", diag.CodeLintControl, ""},
		{"a\u200bb", diag.CodeLintInvisible, "U+200B"},
		{"a\u00a0b", diag.CodeLintInvisible, "U+00A0"},
		{"\ufeffa", diag.CodeLintInvisible, "U+FEFF"},
		{"clean value", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got := LintValue(tt.value)

			if tt.wantCode == "" {
				if len(got) != 0 {
					t.Fatalf("got %+v", got)
				}
				return
			}
			if len(got) != 1 || got[0].Code != tt.wantCode || got[0].Char != tt.wantChar {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestLint_should_check_urls_headers_vars_and_names(t *testing.T) {
	f := newFixture(t)
	c := f.collection("c")
	f.request(&c, "r", "1")
	c.Requests[0].Versions[0].URL = "http://x/a\u200b"
	c.Requests[0].Versions[0].Headers = []model.KeyValue{{Key: "X-Trace ", Value: "ok"}, {Key: "Accept", Value: "a\tb"}}
	c.Environments = []model.Environment{{Meta: model.Meta{ID: f.ids.New()}, File: "/c/c/environments/e.yaml", Vars: []model.EnvVar{{Name: "base", Value: " http://x"}}}}
	c.Manifest.Name = "c "

	got := Validate(Target{Collections: []model.Collection{c}})

	want := map[string]diag.Code{
		"$.versions.1.url":            diag.CodeLintInvisible,
		"$.versions.1.headers":        diag.CodeLintWhitespace,
		"$.versions.1.headers.Accept": diag.CodeLintControl,
		"$.vars.base":                 diag.CodeLintWhitespace,
		"$.name":                      diag.CodeLintWhitespace,
	}
	if len(got) != len(want) {
		t.Fatalf("got %d: %+v", len(got), got)
	}
	for _, d := range got {
		if want[d.Path] != d.Code {
			t.Errorf("unexpected %s at %s", d.Code, d.Path)
		}
	}
}

func TestLint_Given_a_credential_typed_into_a_request_Then_it_warns(t *testing.T) {
	// Before: nothing warned, and the token reached git through the file.
	f := newFixture(t)
	c := f.collection("c")
	f.request(&c, "r", "1")
	c.Requests[0].Versions[0].Headers = []model.KeyValue{
		{Key: "Authorization", Value: "Bearer eyJhbGciOi"},
		{Key: "X-Api-Key", Value: "{{secret.key}}"},
		{Key: "Accept", Value: "application/json"},
	}
	c.Requests[0].Versions[0].Query = []model.KeyValue{{Key: "access_token", Value: "abc123"}, {Key: "page", Value: "2"}}
	c.Environments = []model.Environment{{Meta: model.Meta{ID: f.ids.New()}, File: "/c/c/environments/e.yaml", Vars: []model.EnvVar{
		{Name: "client_secret", Value: "hunter2"},
		{Name: "api_token", Value: "", Secret: true},
		{Name: "base", Value: "http://x"},
	}}}

	got := Validate(Target{Collections: []model.Collection{c}})

	var paths []string
	for _, d := range got {
		if d.Code == diag.CodeLintLiteralKey {
			paths = append(paths, d.Path)
		}
	}
	want := []string{"$.vars.client_secret", "$.versions.1.headers.Authorization", "$.versions.1.query.access_token"}
	slices.Sort(paths)
	if !slices.Equal(paths, want) {
		t.Fatalf("got %v, want %v", paths, want)
	}
}

func TestTypedCredential(t *testing.T) {
	for _, tt := range []struct {
		name, value string
		want        bool
	}{
		{"Authorization", "Bearer abc", true},
		{"Authorization", "Bearer", false},
		{"Authorization", "Bearer {{secret.t}}", false},
		{"Accept", "text/plain", false},
		{"", "abc", false},
	} {
		if got := typedCredential(tt.name, tt.value); got != tt.want {
			t.Errorf("typedCredential(%q, %q) = %v", tt.name, tt.value, got)
		}
	}
}

func TestLint_Given_a_credential_sent_over_plain_http_to_a_remote_host_Then_it_warns(t *testing.T) {
	// Before: nothing warned when a token crossed the internet in clear text.
	tests := map[string]bool{
		"http://api.example.com/x":      true,
		"ws://api.example.com/x":        true,
		"https://api.example.com/x":     false,
		"http://localhost:8080/x":       false,
		"http://127.0.0.1/x":            false,
		"http://192.168.1.5/x":          false,
		"http://db/x":                   false, // one label: a name on the local network
		"{{env.base}}/x":                false, // the host is not known
		"http://{{env.host}}/x":         false,
		"http://203.0.113.9/x":          true,
		"http://api.example.com/{{id}}": true,
	}
	for rawURL, want := range tests {
		f := newFixture(t)
		c := f.collection("c")
		f.request(&c, "r", "1")
		c.Requests[0].Versions[0].URL = rawURL
		c.Requests[0].Versions[0].Headers = []model.KeyValue{{Key: "Authorization", Value: "Bearer {{secret.t}}"}}

		got := false
		for _, d := range Validate(Target{Collections: []model.Collection{c}}) {
			got = got || d.Code == diag.CodeLintPlainKey
		}

		if got != want {
			t.Errorf("%s: warned %v, want %v", rawURL, got, want)
		}
	}
}

func TestLint_Given_plain_http_with_no_credential_Then_it_stays_quiet(t *testing.T) {
	f := newFixture(t)
	c := f.collection("c")
	f.request(&c, "r", "1")
	c.Requests[0].Versions[0].URL = "http://api.example.com/x"
	c.Requests[0].Versions[0].Headers = []model.KeyValue{{Key: "Accept", Value: "application/json"}}

	for _, d := range Validate(Target{Collections: []model.Collection{c}}) {
		if d.Code == diag.CodeLintPlainKey {
			t.Fatalf("unexpected %v", d)
		}
	}
}

func TestLint_Given_a_plaintext_gRPC_call_to_a_remote_host_Then_it_warns(t *testing.T) {
	// Before: a typo from grpcs:// to grpc:// silently changed the security of the call.
	tests := map[string]bool{
		"grpc://api.example.com:443":  true,
		"grpcs://api.example.com:443": false,
		"api.example.com:443":         false, // a bare remote address uses TLS
		"grpc://localhost:50051":      false,
		"grpc://10.0.0.7:50051":       false,
		"grpc://[::1]:50051":          false,
		"{{env.grpc}}":                false,
	}
	for server, want := range tests {
		f := newFixture(t)
		c := f.collection("c")
		f.request(&c, "r", "1")
		c.Requests[0].Versions[0].GRPC = &model.GRPC{Server: server, Method: "a.B/C"}

		got := false
		for _, d := range Validate(Target{Collections: []model.Collection{c}}) {
			got = got || d.Code == diag.CodeLintPlainGRPC
		}

		if got != want {
			t.Errorf("%s: warned %v, want %v", server, got, want)
		}
	}
}
