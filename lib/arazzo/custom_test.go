// SPDX-License-Identifier: Apache-2.0

package arazzo

import (
	"slices"
	"testing"
)

func TestCustomSteps_should_list_the_nodes_other_Arazzo_tools_cannot_run(t *testing.T) {
	f := sampleFlow() // request, assert (folded into the request), set
	f.Nodes = append(f.Nodes, f.Nodes[2])
	f.Nodes[3].ID = "later"
	f.Nodes[3].Type = "parallel"

	got := CustomSteps(f, fakeResolver{})

	var ids []string
	for _, c := range got {
		ids = append(ids, c.Node)
	}
	if !slices.Equal(ids, []string{"remember", "later"}) || got[0].Index != 2 || got[1].Index != 3 || got[1].Type != "parallel" {
		t.Fatalf("got %+v", got)
	}
}

func TestCustomSteps_Given_only_standard_steps_Then_none(t *testing.T) {
	f := sampleFlow()
	f.Nodes = f.Nodes[:2]

	if got := CustomSteps(f, fakeResolver{}); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}
