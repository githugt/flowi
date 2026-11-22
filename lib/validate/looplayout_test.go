// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

func TestCheckParams_Given_a_loop_with_a_layout_When_validated_Then_only_vertical_and_horizontal_pass(t *testing.T) {
	// Before: layout was not a field of a loop, so it was reported as an unknown parameter.
	tests := []struct {
		name string
		node model.Node
		bad  bool
	}{
		{name: "batch_vertical", node: model.Node{ID: "b", Type: "batch", Params: fields("times", int64(2), "layout", "vertical"), Body: []model.Node{note("x")}}},
		{name: "batch_horizontal", node: model.Node{ID: "b", Type: "batch", Params: fields("times", int64(2), "layout", "horizontal"), Body: []model.Node{note("x")}}},
		{name: "batch_without_layout", node: model.Node{ID: "b", Type: "batch", Params: fields("times", int64(2)), Body: []model.Node{note("x")}}},
		{name: "run_till_horizontal", node: model.Node{ID: "r", Type: "run_till", Params: fields("until", "true", "layout", "horizontal"), Body: []model.Node{note("x")}}},
		{name: "batch_sideways", node: model.Node{ID: "b", Type: "batch", Params: fields("times", int64(2), "layout", "sideways"), Body: []model.Node{note("x")}}, bad: true},
		{name: "run_till_diagonal", node: model.Node{ID: "r", Type: "run_till", Params: fields("until", "true", "layout", "diagonal"), Body: []model.Node{note("x")}}, bad: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateFlow(t, nil, tt.node)

			if tt.bad != hasDiag(got, diag.CodeNodeParam, diag.SeverityError) || (!tt.bad && len(got) != 0) {
				t.Fatalf("got %v %v", codesOf(got), got)
			}
		})
	}
}
