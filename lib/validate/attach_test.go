// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"slices"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

func attached(id, to string) model.Node { n := note(id); n.Attach = to; return n }

func call(id string) model.Node { return model.Node{ID: id, Type: "request"} }

func TestAttachProblem_should_accept_only_a_neighbour_request_and_others_attached_to_it(t *testing.T) {
	tests := []struct {
		name string
		list []model.Node
		i    int
		bad  bool
	}{
		{name: "none", list: []model.Node{note("a")}, i: 0},
		{name: "before_the_request", list: []model.Node{attached("a", "r"), call("r")}, i: 0},
		{name: "after_the_request", list: []model.Node{call("r"), attached("a", "r")}, i: 1},
		{name: "two_before", list: []model.Node{attached("a", "r"), attached("b", "r"), call("r")}, i: 0},
		{name: "unknown_target", list: []model.Node{attached("a", "nope"), call("r")}, i: 0, bad: true},
		{name: "target_is_not_a_request", list: []model.Node{attached("a", "b"), note("b")}, i: 0, bad: true},
		{name: "another_step_between", list: []model.Node{attached("a", "r"), note("x"), call("r")}, i: 0, bad: true},
		{name: "drawn_above", list: []model.Node{{ID: "a", Type: "log", Attach: "r", AttachAt: "above"}, call("r")}, i: 0},
		{name: "drawn_below_after", list: []model.Node{call("r"), {ID: "a", Type: "log", Attach: "r", AttachAt: "below"}}, i: 1},
		{name: "drawn_somewhere_else", list: []model.Node{{ID: "a", Type: "log", Attach: "r", AttachAt: "left"}, call("r")}, i: 0, bad: true},
		{name: "drawn_without_attach", list: []model.Node{{ID: "a", Type: "log", AttachAt: "above"}}, i: 0, bad: true},
		{name: "request_attached_to_request", list: []model.Node{{ID: "q", Type: "request", Attach: "r"}, call("r")}, i: 0, bad: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := attachProblem(tt.list, tt.i); (got != "") != tt.bad {
				t.Fatalf("problem %q, want bad=%v", got, tt.bad)
			}
		})
	}
}

func TestValidate_should_warn_about_an_attach_that_cannot_be_drawn_and_not_fail_the_flow(t *testing.T) {
	got := validateFlow(t, nil, attached("a", "nope"), note("b"))

	if !hasDiag(got, diag.CodeNodeAttach, diag.SeverityWarning) || slices.ContainsFunc(got, func(d diag.Diagnostic) bool { return d.Code == diag.CodeNodeAttach && d.Severity == diag.SeverityError }) {
		t.Fatalf("got %v", got)
	}
}
