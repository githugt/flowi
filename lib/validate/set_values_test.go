// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

func TestNodes_Given_a_removed_node_type_Then_the_diagnostic_names_the_replacement(t *testing.T) {
	for _, removed := range []model.NodeType{"extract", "generate", "transform", "calc"} {
		t.Run(string(removed), func(t *testing.T) {
			f := newFixture(t)
			c := f.collection("c")
			f.flow(&c, "x", model.Node{ID: "a", Type: removed})

			got := Validate(Target{Collections: []model.Collection{c}})

			d := only(t, got, diag.CodeNodeRemovedType)
			if d.Path != "$.nodes[0].type" || !strings.Contains(d.Args["use"], "set") {
				t.Fatalf("path %q use %q", d.Path, d.Args["use"])
			}
		})
	}
}

func TestNodes_Given_two_values_writing_one_name_When_validated_Then_a_problem_is_reported(t *testing.T) {
	f := newFixture(t)
	c := f.collection("c")
	f.flow(&c, "x", model.Node{ID: "a", Type: "set", Values: []model.SetValue{
		{Name: "id", Value: "{{1}}"},
		{Name: "id", Value: "{{2}}"},
	}})

	got := Validate(Target{Collections: []model.Collection{c}})

	d := only(t, got, diag.CodeNodeParam)
	if d.Path != "$.nodes[0].values[1].name" {
		t.Fatalf("path %q", d.Path)
	}
}

func TestNodes_Given_a_value_that_is_incomplete_When_validated_Then_a_problem_names_what_is_missing(t *testing.T) {
	tests := []struct {
		name  string
		value model.SetValue
		path  string
	}{
		{"no_name", model.SetValue{Value: "{{1}}"}, "$.nodes[0].values[0].name"},
		{"unknown_source", model.SetValue{Name: "a", From: "magic"}, "$.nodes[0].values[0].from"},
		{"response_without_place", model.SetValue{Name: "a", From: "response"}, "$.nodes[0].values[0].in"},
		{"header_without_name", model.SetValue{Name: "a", From: "response", Where: "header"}, "$.nodes[0].values[0].header"},
		{"body_without_path", model.SetValue{Name: "a", From: "response", Where: "body"}, "$.nodes[0].values[0].path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			c := f.collection("c")
			f.flow(&c, "x", model.Node{ID: "a", Type: "set", Values: []model.SetValue{tt.value}})

			got := Validate(Target{Collections: []model.Collection{c}})

			d := only(t, got, diag.CodeNodeParam)
			if d.Path != tt.path {
				t.Fatalf("path %q, want %q", d.Path, tt.path)
			}
		})
	}
}

func TestNodes_Given_a_colour_not_in_the_list_Then_a_problem_is_reported(t *testing.T) {
	f := newFixture(t)
	c := f.collection("c")
	f.flow(&c, "x", model.Node{ID: "a", Type: "note", Color: "chartreuse", Params: params("text", "x")})

	got := Validate(Target{Collections: []model.Collection{c}})

	d := only(t, got, diag.CodeNodeParam)
	if d.Path != "$.nodes[0].color" {
		t.Fatalf("path %q", d.Path)
	}
}

func TestNodes_Given_a_colour_in_the_list_Then_no_problem(t *testing.T) {
	f := newFixture(t)
	c := f.collection("c")
	f.flow(&c, "x", model.Node{ID: "a", Type: "note", Color: "teal", Params: params("text", "x")})

	got := Validate(Target{Collections: []model.Collection{c}})

	if len(got) != 0 {
		t.Fatalf("got %v", codesOf(got))
	}
}
