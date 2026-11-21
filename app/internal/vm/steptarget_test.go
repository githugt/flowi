// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
)

func TestStepTargetID_should_name_the_request_or_flow_a_step_calls(t *testing.T) {
	t.Parallel()
	// Before: a step could not be followed to its request or sub-flow; a double-click only collapsed strips.
	cases := []struct {
		name string
		n    model.Node
		want string
	}{
		{"request in the same collection", model.Node{Request: &model.RequestCall{Request: model.Ref{ID: "R1"}}}, "r:C1:R1"},
		{"request of another collection", model.Node{Request: &model.RequestCall{Request: model.Ref{Collection: "C2", ID: "R9"}}}, "r:C2:R9"},
		{"subflow", model.Node{Subflow: &model.SubflowCall{Flow: model.Ref{ID: "F1"}}}, "f:C1:F1"},
		{"blueprint parameter", model.Node{Request: &model.RequestCall{Request: model.Ref{Param: "target"}}}, ""},
		{"another kind of step", model.Node{}, ""},
	}
	for _, c := range cases {
		if got := vm.StepTargetID("C1", c.n); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
