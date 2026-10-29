// SPDX-License-Identifier: Apache-2.0

package model

import (
	"slices"
	"testing"
)

func TestFlowTags_Given_own_and_folder_tags_Then_a_flow_carries_both_without_repeats(t *testing.T) {
	// Arrange
	c := Collection{
		Folders: []Folder{{Dir: "flows", Tags: []string{"all"}}, {Dir: "flows/billing", Tags: []string{"slow", "all"}}},
		Flows:   []Flow{{Path: "billing/refund", Tags: []string{"smoke"}}},
	}

	// Act
	got := c.FlowTags(c.Flows[0])

	// Assert
	want := []string{"all", "slow", "smoke"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestFlowsTagged_Given_a_tag_Then_only_flows_that_carry_it_are_listed_in_file_order(t *testing.T) {
	// Arrange
	c := Collection{
		Folders: []Folder{{Dir: "flows/billing", Tags: []string{"slow"}}},
		Flows: []Flow{
			{Path: "billing/refund"},
			{Path: "login", Tags: []string{"smoke"}},
			{Path: "search"},
		},
	}

	// Act
	slow := c.FlowsTagged("slow")
	smoke := c.FlowsTagged("smoke")
	none := c.FlowsTagged("missing")

	// Assert
	if len(slow) != 1 || slow[0].Path != "billing/refund" || len(smoke) != 1 || smoke[0].Path != "login" || len(none) != 0 {
		t.Fatalf("slow %v smoke %v none %v", slow, smoke, none)
	}
}
