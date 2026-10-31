// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"reflect"
	"strings"
	"testing"

	flowi "github.com/githugt/flowi/lib"

	"github.com/githugt/flowi/app/internal/vm"
)

func kinds(lines []vm.BodyLine) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(map[string]string{vm.LineSame: " ", vm.LineAdd: "+", vm.LineDel: "-"}[l.Kind] + l.Text + "\n")
	}
	return b.String()
}

func TestLineDiff_should_keep_common_lines_and_mark_the_rest(t *testing.T) {
	// Arrange
	before := []string{"a", "b", "c", "d"}
	after := []string{"a", "c", "x", "d"}

	// Act
	got := kinds(vm.LineDiff(before, after))

	// Assert
	if want := " a\n-b\n c\n+x\n d\n"; got != want {
		t.Fatalf("got\n%s want\n%s", got, want)
	}
}

func TestLineDiff_Given_nothing_before_Then_every_line_is_added(t *testing.T) {
	got := kinds(vm.LineDiff(nil, []string{"a", "b"}))

	if got != "+a\n+b\n" {
		t.Fatalf("got %q", got)
	}
}

func TestLineDiff_Given_a_huge_body_Then_shows_it_replaced_instead_of_stalling(t *testing.T) {
	// Arrange
	big := make([]string, 5000)
	for i := range big {
		big[i] = "line"
	}

	// Act
	got := vm.LineDiff(big, big)

	// Assert
	if len(got) != 10000 || got[0].Kind != vm.LineDel || got[len(got)-1].Kind != vm.LineAdd {
		t.Fatalf("%d lines, first %s", len(got), got[0].Kind)
	}
}

func TestOutputLines_should_leave_out_the_time_a_node_took(t *testing.T) {
	got := vm.OutputLines(map[string]any{"status": 200, "time": 123})

	if want := []string{"{", `  "status": 200`, "}"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q", got)
	}
}

func TestCompareDetails_should_list_only_nodes_whose_outputs_differ_and_the_variables_that_moved(t *testing.T) {
	// Arrange
	before := flowi.RunDetail{
		Outputs: map[string]map[string]any{"login": {"status": 200, "time": 10}, "list": {"status": 200, "body": "a"}, "gone": {"status": 1}},
		Vars:    map[string]any{"token": "abc", "same": "x", "dropped": 1},
	}
	after := flowi.RunDetail{
		Outputs: map[string]map[string]any{"login": {"status": 200, "time": 99}, "list": {"status": 200, "body": "b"}},
		Vars:    map[string]any{"token": "def", "same": "x", "added": true},
	}

	// Act
	got := vm.CompareDetails(before, after)

	// Assert
	if _, same := got.Nodes["login"]; same {
		t.Error("a node that differs only in its time did not change")
	}
	if len(got.Nodes["list"]) == 0 || len(got.Nodes["gone"]) == 0 {
		t.Errorf("nodes %v", got.Nodes)
	}
	names := []string{}
	for _, v := range got.Vars {
		names = append(names, v.Name)
	}
	if !reflect.DeepEqual(names, []string{"added", "dropped", "token"}) {
		t.Errorf("variables that moved: %v", names)
	}
}
