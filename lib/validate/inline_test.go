// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

func params(kv ...string) []model.Field {
	var out []model.Field
	for i := 0; i < len(kv); i += 2 {
		out = append(out, model.Field{Key: kv[i], Value: kv[i+1]})
	}
	return out
}

func TestValidate_Given_inline_node_missing_param_Then_FLW_NODE_004(t *testing.T) {
	tests := []struct {
		node  model.Node
		field string
	}{
		{model.Node{ID: "s", Type: "set", Params: params("value", "x")}, "values"},
		{model.Node{ID: "s", Type: "set", Params: params("target", "vars.x")}, "value"},
		{model.Node{ID: "a", Type: "assert"}, "check"},
		{model.Node{ID: "l", Type: "log"}, "value"},
	}
	for _, tt := range tests {
		t.Run(string(tt.node.Type)+"/"+tt.field, func(t *testing.T) {
			f := newFixture(t)
			c := f.collection("c")
			c.Manifest.Hooks.Post = []model.HookStep{{Inline: []model.Node{tt.node}}}

			got := Validate(Target{Collections: []model.Collection{c}})

			d := only(t, got, diag.CodeNodeMissingField)
			if d.Args["field"] != tt.field {
				t.Fatalf("field %q", d.Args["field"])
			}
		})
	}
}

func TestValidate_Given_set_values_Then_no_missing_field(t *testing.T) {
	f := newFixture(t)
	c := f.collection("c")
	c.Manifest.Hooks.Post = []model.HookStep{{Inline: []model.Node{{ID: "e", Type: "set", Values: []model.SetValue{{Name: "token", Value: "{{response.body.token}}"}}}}}}

	got := Validate(Target{Collections: []model.Collection{c}})

	if len(got) != 0 {
		t.Fatalf("got %v", codesOf(got))
	}
}

func TestValidate_Given_unknown_set_target_Then_FLW_RUN_004_warning(t *testing.T) {
	f := newFixture(t)
	c := f.collection("c")
	c.Manifest.Hooks.Pre = []model.HookStep{{Inline: []model.Node{{ID: "s", Type: "set", Params: params("target", "headers.X", "value", "v")}}}}

	got := Validate(Target{Collections: []model.Collection{c}})

	d := only(t, got, diag.CodeRunSetTarget)
	if d.Severity != diag.SeverityWarning || d.Path != "$.hooks.pre[0].inline[0].target" {
		t.Fatalf("diag %+v", d)
	}
}

func TestValidate_Given_an_assert_with_only_a_schema_Then_it_is_complete(t *testing.T) {
	// Arrange
	f := newFixture(t)
	c := f.collection("c")
	c.Manifest.Hooks.Post = []model.HookStep{{Inline: []model.Node{{ID: "a", Type: "assert", Params: params("schema", "login")}}}}

	// Act
	got := Validate(Target{Collections: []model.Collection{c}})

	// Assert
	for _, d := range got {
		if d.Code == diag.CodeNodeMissingField {
			t.Fatalf("an assert with a schema needs no check: %v", d)
		}
	}
}
