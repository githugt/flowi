// SPDX-License-Identifier: Apache-2.0

package schemainfer

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// canon turns a schema into plain JSON values so []string and []any compare equal.
func canon(t *testing.T, v any) any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func decode(t *testing.T, text string) any {
	t.Helper()
	d := json.NewDecoder(strings.NewReader(text))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestInfer_should_describe_one_sample(t *testing.T) {
	tests := []struct{ name, sample, want string }{
		{"string", `"hi"`, `{"type":"string"}`},
		{"integer", `132`, `{"type":"integer"}`},
		{"number", `1.5`, `{"type":"number"}`},
		{"bool", `true`, `{"type":"boolean"}`},
		{"null", `null`, `{"type":"null"}`},
		{"object", `{"id":1,"name":"x","tags":[]}`,
			`{"type":"object","properties":{"id":{"type":"integer"},"name":{"type":"string"},"tags":{"type":"array"}},"required":["id","name","tags"]}`},
		{"empty object", `{}`, `{"type":"object","properties":{}}`},
		{"array of ints", `[1,2,3]`, `{"type":"array","items":{"type":"integer"}}`},
		{"array of objects with one shape", `[{"a":1},{"a":2}]`,
			`{"type":"array","items":{"type":"object","properties":{"a":{"type":"integer"}},"required":["a"]}}`},
		{"array of mixed object shapes", `[{"a":1,"b":"x"},{"a":2}]`,
			`{"type":"array","items":{"type":"object","properties":{"a":{"type":"integer"},"b":{"type":"string"}},"required":["a"]}}`},
		{"array of ints and floats", `[1,2.5]`, `{"type":"array","items":{"type":"number"}}`},
		{"nested", `{"a":{"b":[{"c":null}]}}`,
			`{"type":"object","properties":{"a":{"type":"object","properties":{"b":{"type":"array","items":{"type":"object","properties":{"c":{"type":"null"}},"required":["c"]}}},"required":["b"]}},"required":["a"]}`},
		{"date-time", `"2026-10-15T10:00:00Z"`, `{"type":"string","format":"date-time"}`},
		{"uuid", `"123e4567-e89b-12d3-a456-426614174000"`, `{"type":"string","format":"uuid"}`},
		{"email", `"ada@example.com"`, `{"type":"string","format":"email"}`},
		{"uri", `"https://example.com/x"`, `{"type":"string","format":"uri"}`},
		{"not an email", `"a@b"`, `{"type":"string"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Infer(decode(t, tt.sample))

			if !reflect.DeepEqual(canon(t, got), canon(t, decode(t, tt.want))) {
				gotJSON, _ := json.Marshal(got)
				t.Fatalf("got  %s\nwant %s", gotJSON, tt.want)
			}
		})
	}
}

// A body decoded without json.Number gives float64: a whole number counts as an integer.
func TestInfer_should_treat_a_whole_float64_as_an_integer(t *testing.T) {
	got := Infer(float64(132))

	if got["type"] != "integer" {
		t.Fatalf("got %v", got)
	}
}

func TestInfer_Given_a_body_beyond_the_bounds_Then_marks_the_schema_truncated(t *testing.T) {
	deep := any("leaf")
	for range MaxDepth + 5 {
		deep = map[string]any{"n": deep}
	}
	wide := make([]any, MaxNodes+10)
	for i := range wide {
		wide[i] = map[string]any{"k": json.Number("1")}
	}

	for name, sample := range map[string]any{"deep": deep, "wide": wide} {
		got := Infer(sample)

		if got["x-flowi-truncated"] != true {
			t.Errorf("%s: not marked truncated: %v", name, got)
		}
	}
}

// Before: required came out as []string, which a JSON Schema validator
// rejects ("invalid jsonType []string"); schemas must hold decoded-JSON shapes only.
func TestInfer_should_produce_only_decoded_JSON_shapes(t *testing.T) {
	got := Infer(decode(t, `{"a":[{"b":1}]}`))

	if _, ok := got["required"].([]any); !ok {
		t.Fatalf("required is %T", got["required"])
	}
	if _, ok := got["properties"].(map[string]any); !ok {
		t.Fatalf("properties is %T", got["properties"])
	}
}
