// SPDX-License-Identifier: Apache-2.0

package model

import (
	"slices"
	"testing"
)

func TestNode_Children_should_return_only_non_empty_child_lists(t *testing.T) {
	a, b, c := Node{ID: "a"}, Node{ID: "b"}, Node{ID: "c"}
	n := Node{
		Body:     []Node{a},
		Branches: []Branch{{ID: "x", Nodes: []Node{b}}, {ID: "empty"}},
		Then:     []Node{c},
		Cases:    []Case{{When: "true"}},
	}

	got := n.Children()

	if len(got) != 3 {
		t.Fatalf("got %d child lists, want 3", len(got))
	}
	if got[0][0].ID != "a" || got[1][0].ID != "b" || got[2][0].ID != "c" {
		t.Fatalf("unexpected order: %+v", got)
	}
}

func TestNodeTypes_should_list_17_types_in_spec_order(t *testing.T) {
	want := []NodeType{
		"request", "fetch", "webhook", "subflow", "set",
		"assert", "fuzz", "condition", "delay", "batch", "run_till", "parallel", "stop", "log", "note", "digest", "save",
	}

	got := NodeTypes()

	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
