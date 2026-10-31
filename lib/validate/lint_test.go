// SPDX-License-Identifier: Apache-2.0

package validate

import (
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
