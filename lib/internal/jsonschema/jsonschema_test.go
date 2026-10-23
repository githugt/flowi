// SPDX-License-Identifier: Apache-2.0

package jsonschema

import "testing"

// Before: the outer schema was merged by recursing on itself (it still had
// allOf), which never ended.
func TestMergeAllOf_should_combine_branches_and_the_outer_schema(t *testing.T) {
	s := map[string]any{
		"description": "pet",
		"allOf": []any{
			map[string]any{"properties": map[string]any{"name": map[string]any{}}, "required": []any{"name"}},
			map[string]any{"properties": map[string]any{"id": map[string]any{}}},
		},
	}

	got := MergeAllOf(s)

	props, _ := got["properties"].(map[string]any)
	if len(props) != 2 || got["description"] != "pet" || got["type"] != "object" || len(got["required"].([]any)) != 1 {
		t.Fatalf("got %v", got)
	}
}

// Before: a free-form object, additionalProperties: true and properties
// beside oneOf all gave Absent (✗) for reads the schema allows.
func TestLookup_should_not_call_allowed_reads_absent(t *testing.T) {
	obj := func(kv ...any) map[string]any {
		m := map[string]any{}
		for i := 0; i < len(kv); i += 2 {
			m[kv[i].(string)] = kv[i+1]
		}
		return m
	}
	idOnly := obj("type", "object", "properties", obj("id", obj("type", "integer")))
	cases := []struct {
		name   string
		schema map[string]any
		steps  []string
		want   Result
	}{
		{"free-form object", obj("type", "object"), []string{"x"}, Unknown},
		{"additionalProperties true", obj("type", "object", "properties", obj("id", obj()), "additionalProperties", true), []string{"x"}, Unknown},
		{"additionalProperties false", obj("type", "object", "properties", obj("id", obj()), "additionalProperties", false), []string{"x"}, Absent},
		{"listed properties", idOnly, []string{"x"}, Absent},
		{"base properties beside oneOf", obj("type", "object", "properties", obj("id", obj("type", "integer")),
			"oneOf", []any{obj("properties", obj("a", obj())), obj("properties", obj("b", obj()))}), []string{"id"}, Present},
		{"a branch property beside base properties", obj("type", "object", "properties", obj("id", obj()),
			"oneOf", []any{obj("properties", obj("a", obj()))}), []string{"a"}, Present},
		{"in no branch", obj("type", "object", "properties", obj("id", obj()),
			"oneOf", []any{obj("properties", obj("a", obj()))}), []string{"zzz"}, Absent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Lookup(tc.schema, tc.steps); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
