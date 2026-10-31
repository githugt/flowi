// SPDX-License-Identifier: Apache-2.0

package schemainfer

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func TestMerge_should_widen_a_schema_and_report_each_change(t *testing.T) {
	tests := []struct {
		name         string
		first, next  string
		want         string
		wantKinds    []ChangeKind
		wantFirstHit string // path of the first change
	}{
		{"missing field becomes optional", `{"id":1,"name":"x"}`, `{"id":2}`,
			`{"type":"object","properties":{"id":{"type":"integer"},"name":{"type":"string"}},"required":["id"]}`,
			[]ChangeKind{OptionalField}, "$.name"},
		{"new field", `{"id":1}`, `{"id":2,"extra":true}`,
			`{"type":"object","properties":{"id":{"type":"integer"},"extra":{"type":"boolean"}},"required":["id"]}`,
			[]ChangeKind{NewField}, "$.extra"},
		{"integer then float", `{"n":1}`, `{"n":1.5}`,
			`{"type":"object","properties":{"n":{"type":"number"}},"required":["n"]}`,
			[]ChangeKind{TypeWidened}, "$.n"},
		{"float then whole number", `{"n":1.5}`, `{"n":2}`,
			`{"type":"object","properties":{"n":{"type":"number"}},"required":["n"]}`, nil, ""},
		{"value then null", `{"a":"x"}`, `{"a":null}`,
			`{"type":"object","properties":{"a":{"type":["string","null"]}},"required":["a"]}`,
			[]ChangeKind{NowNullable}, "$.a"},
		{"identical sample", `{"a":1,"b":[1,2]}`, `{"a":5,"b":[3]}`,
			`{"type":"object","properties":{"a":{"type":"integer"},"b":{"type":"array","items":{"type":"integer"}}},"required":["a","b"]}`, nil, ""},
		{"arrays of different object shapes keep one items schema", `[{"a":1,"b":2}]`, `[{"a":1},{"a":2,"c":3}]`,
			`{"type":"array","items":{"type":"object","properties":{"a":{"type":"integer"},"b":{"type":"integer"},"c":{"type":"integer"}},"required":["a"]}}`,
			[]ChangeKind{OptionalField, NewField}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existing := Infer(decode(t, tt.first))

			got, changes := Merge(existing, decode(t, tt.next))

			if !reflect.DeepEqual(canon(t, got), canon(t, decode(t, tt.want))) {
				gotJSON, _ := json.Marshal(got)
				t.Fatalf("got  %s\nwant %s", gotJSON, tt.want)
			}
			var kinds []ChangeKind
			for _, c := range changes {
				kinds = append(kinds, c.Kind)
			}
			slices.Sort(kinds)
			want := slices.Clone(tt.wantKinds)
			slices.Sort(want)
			if !slices.Equal(kinds, want) {
				t.Fatalf("changes %+v, want kinds %v", changes, want)
			}
			if tt.wantFirstHit != "" && changes[0].Path != tt.wantFirstHit {
				t.Fatalf("first change at %q, want %q", changes[0].Path, tt.wantFirstHit)
			}
		})
	}
}

// A schema that came from an OpenAPI import, or that the user edited, is not ours to rewrite.
func TestMerge_should_leave_schemas_from_a_spec_or_the_user_alone(t *testing.T) {
	for _, source := range []string{"openapi", "user"} {
		existing := Schema{"type": "object", "x-flowi-source": source, "properties": map[string]any{"a": map[string]any{"type": "string"}}}

		got, changes := Merge(existing, decode(t, `{"a":1,"b":2}`))

		if !reflect.DeepEqual(canon(t, got), canon(t, existing)) || len(changes) != 0 {
			t.Errorf("%s: schema changed: %v %+v", source, got, changes)
		}
	}
}
