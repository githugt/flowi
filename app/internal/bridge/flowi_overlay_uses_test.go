// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

func stepWithOverlay(overlay string) model.Node {
	return model.Node{Type: "request", Request: &model.RequestCall{Overlay: overlay}}
}

func TestFlowsUsingOverlay_Given_steps_in_nested_lists_When_listed_Then_each_flow_appears_once_sorted(t *testing.T) {
	// Before: deleting an overlay did not say which flows named it, so they found out when they next ran.
	c := model.Collection{Flows: []model.Flow{
		{Path: "b-checkout", Nodes: []model.Node{stepWithOverlay("vip"), stepWithOverlay("vip")}},
		{Path: "a-signup", Nodes: []model.Node{{Type: "batch", Body: []model.Node{{Type: "condition", Then: []model.Node{stepWithOverlay("vip")}}}}}},
		{Path: "c-other", Nodes: []model.Node{stepWithOverlay("slow")}},
		{Path: "d-none", Nodes: []model.Node{stepWithOverlay("")}},
	}}

	got := flowsUsingOverlay(c, "vip")

	if want := []string{"a-signup", "b-checkout"}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestFlowsUsingOverlay_Given_no_flow_uses_it_When_listed_Then_the_list_is_empty_not_null(t *testing.T) {
	got := flowsUsingOverlay(model.Collection{Flows: []model.Flow{{Path: "x", Nodes: []model.Node{stepWithOverlay("slow")}}}}, "vip")

	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v", got)
	}
}
