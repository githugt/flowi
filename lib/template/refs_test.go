// SPDX-License-Identifier: Apache-2.0

package template_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/template"
)

func TestReferences_should_list_constant_chains(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		expression bool
		want       [][]string
	}{
		{name: "template", src: "Bearer {{nodes.login.token}}", want: [][]string{{"nodes", "login", "token"}}},
		{name: "coalesce", src: "vars.a ?? inputs.b", expression: true, want: [][]string{{"vars", "a"}, {"inputs", "b"}}},
		{name: "dedup_in_source_order", src: "{{vars.a}}-{{env.x}}-{{vars.a}}", want: [][]string{{"vars", "a"}, {"env", "x"}}},
		{name: "index_steps", src: "{{nodes.list.body.items[0].id}}", want: [][]string{{"nodes", "list", "body", "items", "0", "id"}}},
		{name: "non_constant_step_stops", src: "{{nodes[x].y}}", want: [][]string{{"nodes"}, {"x"}}},
		{name: "generator_only", src: "{{$uuid-v4}}", want: nil},
		{name: "literal_text", src: "plain", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs, err := template.References(tt.src, tt.expression)

			if err != nil {
				t.Fatal(err)
			}
			var got [][]string
			for _, r := range refs {
				got = append(got, r.Path)
			}
			if !slices.EqualFunc(got, tt.want, slices.Equal[[]string]) {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReferences_Given_syntax_error_Then_SyntaxError(t *testing.T) {
	_, err := template.References("{{ a + }}", false)

	var se *template.SyntaxError
	if !errors.As(err, &se) {
		t.Fatalf("got %v", err)
	}
}

func TestReferences_should_mark_reads_guarded_only_when_every_use_is_guarded(t *testing.T) {
	got, err := template.References(`{{a.b ?? 'x'}} {{c?.d}} {{e.f}} {{e.f ?? 1}}`, false)

	guarded := map[string]bool{}
	for _, r := range got {
		guarded[strings.Join(r.Path, ".")] = r.Guarded
	}
	if err != nil || !guarded["a.b"] || !guarded["c.d"] || guarded["e.f"] {
		t.Fatalf("got %+v %v, want a.b and c.d guarded, e.f not", got, err)
	}
}
