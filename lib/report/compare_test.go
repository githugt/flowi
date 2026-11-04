// SPDX-License-Identifier: Apache-2.0

package report

import (
	"strings"
	"testing"
)

func compared(wrote2 string, status2 string, extra bool) (Run, Run) {
	a := Run{Nodes: []Node{
		{ID: "login", Status: "passed"},
		{ID: "grab", Status: "passed", Wrote: []VarWrite{{Name: "token", Value: "t-1"}}},
		{ID: "verify", Status: "passed"},
	}}
	b := Run{Nodes: []Node{
		{ID: "login", Status: "passed"},
		{ID: "grab", Status: "passed", Wrote: []VarWrite{{Name: "token", Value: wrote2}}},
		{ID: "verify", Status: status2},
	}}
	if extra {
		b.Nodes = append(b.Nodes, Node{ID: "cleanup", Status: "passed"})
	}
	return a, b
}

func TestCompare_Given_identical_runs_Then_no_changes(t *testing.T) {
	a, b := compared("t-1", "passed", false)

	got := Compare(a, b)

	if len(got.Changes) != 0 || got.FirstDivergence != "" {
		t.Fatalf("%+v", got)
	}
}

func TestCompare_should_name_the_first_step_that_went_differently_and_every_change(t *testing.T) {
	a, b := compared("t-2", "failed", true)

	got := Compare(a, b)

	if got.FirstDivergence != "grab" {
		t.Errorf("first divergence %q", got.FirstDivergence)
	}
	want := []Change{
		{Node: "grab", What: "wrote token", From: "t-1", To: "t-2"},
		{Node: "verify", What: "status", From: "passed", To: "failed"},
		{Node: "cleanup", What: "step", From: "missing", To: "ran"},
	}
	if len(got.Changes) != len(want) {
		t.Fatalf("changes %+v", got.Changes)
	}
	for i, w := range want {
		if got.Changes[i] != w {
			t.Errorf("change %d = %+v, want %+v", i, got.Changes[i], w)
		}
	}
}

func TestWriteComparison_should_say_when_the_runs_match_and_list_changes_otherwise(t *testing.T) {
	a, b := compared("t-2", "passed", false)
	var same, differ strings.Builder

	WriteComparison(&same, Compare(a, a))
	WriteComparison(&differ, Compare(a, b))

	if !strings.Contains(same.String(), "same") {
		t.Errorf("same: %q", same.String())
	}
	if !strings.Contains(differ.String(), "first differs at grab") || !strings.Contains(differ.String(), "t-1 → t-2") {
		t.Errorf("differ: %q", differ.String())
	}
}
