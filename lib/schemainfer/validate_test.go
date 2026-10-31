// SPDX-License-Identifier: Apache-2.0

package schemainfer

import (
	"errors"
	"strings"
	"testing"
)

func TestValidate_should_report_where_a_sample_breaks_its_schema(t *testing.T) {
	schema := Infer(decode(t, `{"id":1,"name":"x","tags":["a"],"when":"2026-10-15T10:00:00Z"}`))
	tests := []struct {
		name, sample string
		wantPaths    []string
	}{
		{"fits", `{"id":2,"name":"y","tags":[],"when":"2026-10-16T10:00:00Z","extra":1}`, nil},
		{"missing required field", `{"id":2,"tags":[],"when":"2026-10-16T10:00:00Z"}`, []string{"$"}},
		{"wrong type", `{"id":"2","name":"y","tags":[],"when":"2026-10-16T10:00:00Z"}`, []string{"$.id"}},
		{"wrong element type", `{"id":2,"name":"y","tags":[3],"when":"2026-10-16T10:00:00Z"}`, []string{"$.tags[0]"}},
		{"format mismatch", `{"id":2,"name":"y","tags":[],"when":"yesterday"}`, []string{"$.when"}},
		{"not an object", `null`, []string{"$"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vs, err := Validate(schema, decode(t, tt.sample))

			if err != nil {
				t.Fatal(err)
			}
			var paths []string
			for _, v := range vs {
				paths = append(paths, v.Path)
				if v.Message == "" {
					t.Errorf("violation at %s has no message", v.Path)
				}
			}
			if strings.Join(paths, ",") != strings.Join(tt.wantPaths, ",") {
				t.Fatalf("paths %v, want %v (%+v)", paths, tt.wantPaths, vs)
			}
		})
	}
}

func TestValidate_Given_a_schema_that_does_not_compile_Then_ErrBadSchema(t *testing.T) {
	_, err := Validate(Schema{"type": "nonsense"}, decode(t, `1`))

	if !errors.Is(err, ErrBadSchema) {
		t.Fatalf("err %v", err)
	}
}
