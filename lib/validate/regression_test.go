// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

// A blank header row crashed the CLI. Flow-i refuses the
// request instead of skipping the row, so a stray row cannot sit unnoticed
// in the file.
func TestValidate_Given_empty_header_name_When_validated_Then_FLW_REQ_005(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"empty", ""},
		{"space inside", "X Bad"},
		{"template", "{{vars.h}}"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			c := f.collection("c")
			f.request(&c, "r", "1")
			c.Requests[0].Versions[0].Headers = []model.KeyValue{{Key: "Accept", Value: "x"}, {Key: tc.key, Value: "v"}}

			got := checkHeaderNames(buildIndex([]model.Collection{c}))

			if len(got) != 1 || got[0].Code != diag.CodeRequestHeader {
				t.Fatalf("got %v", got)
			}
		})
	}
}

func TestValidate_Given_valid_header_names_When_validated_Then_no_diagnostics(t *testing.T) {
	f := newFixture(t)
	c := f.collection("c")
	f.request(&c, "r", "1")
	c.Requests[0].Versions[0].Headers = []model.KeyValue{{Key: "Accept", Value: "x"}, {Key: "X-Trace-Id", Value: "1"}}

	got := checkHeaderNames(buildIndex([]model.Collection{c}))

	if len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}
