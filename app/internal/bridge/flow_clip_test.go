// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

// openClip opens the signup flow (steps a and b) on a backend the test keeps, to set what a paste parses to.
func openClip(t *testing.T) (*bridge.FlowService, bridge.FlowView, *published, *vmtest.Backend) {
	t.Helper()
	c := vmtest.SampleCollection()
	c.Flows = []model.Flow{{Meta: model.Meta{ID: "F1", Name: "Signup"}, Path: "signup", File: "/c/flows/signup.arazzo.yaml", Nodes: []model.Node{
		{ID: "a", Type: "log"},
		{ID: "b", Type: "log", Needs: []string{"a"}},
	}}}
	pub := &published{}
	b := &vmtest.Backend{Cols: []model.Collection{c}}
	app := bridge.NewApp(b, bridge.NewInlineActor(), pub.emit)
	app.Start(t.Context())
	flow, _ := findNode(app.Workspace().Tree().Roots, "flow")
	v, err := app.Flows().Open(flow.ID)
	if err != nil {
		t.Fatal(err)
	}
	return app.Flows(), v, pub, b
}

func TestFlowService_SelectAll_Given_a_flow_Then_every_top_step_is_selected_with_the_last_primary(t *testing.T) {
	// Before: only Shift-click could select several steps, one at a time.
	flows, v, pub, _ := openClip(t)

	flows.SelectAll(v.Node)

	got := lastViewOr(pub, v)
	if !slices.Equal(got.Selection, []string{"a", "b"}) || got.Selected != "b" {
		t.Fatalf("selection %v, primary %q", got.Selection, got.Selected)
	}
}

func TestFlowService_CopySteps_Given_steps_Then_it_is_their_text_and_given_none_it_fails(t *testing.T) {
	flows, v, _, _ := openClip(t)

	text, err := flows.CopySteps(v.Node, []string{"a", "b"})
	_, none := flows.CopySteps(v.Node, nil)

	if err != nil || text != "nodes: 2\n" || none == nil {
		t.Fatalf("text %q, err %v, none %v", text, err, none)
	}
}

func TestFlowService_PasteSteps_Given_steps_text_Then_they_follow_the_selected_step_and_are_selected_and_undo_removes_them(t *testing.T) {
	flows, v, pub, b := openClip(t)
	b.ParsedFlow = &model.Flow{Nodes: []model.Node{{ID: "a", Type: "log"}, {ID: "n", Type: "log"}}}
	flows.Select(v.Node, "a")

	pasted, err := flows.PasteSteps(v.Node, "text")
	ids := pasted.IDs

	got := lastViewOr(pub, v)
	if err != nil || len(ids) != 2 || !slices.Equal(got.Selection, []string{ids[0], ids[1]}) {
		t.Fatalf("ids %v, err %v, selection %v", ids, err, got.Selection)
	}
	flows.Undo(v.Node)
	if after := lastViewOr(pub, v); len(after.Diagram.Units) != 2 {
		t.Errorf("after undo the flow has %d steps", len(after.Diagram.Units))
	}
}

func TestFlowService_PasteSteps_Given_text_that_is_not_steps_Then_it_fails_and_the_flow_is_unchanged(t *testing.T) {
	flows, v, pub, _ := openClip(t)

	pasted, err := flows.PasteSteps(v.Node, "https://example.com")
	ids := pasted.IDs

	if err == nil || ids != nil || len(lastViewOr(pub, v).Diagram.Units) != len(v.Diagram.Units) {
		t.Fatalf("ids %v, err %v", ids, err)
	}
}

func TestFlowService_PasteSteps_Given_a_request_this_collection_lacks_Then_the_steps_are_pasted_and_it_is_reported(t *testing.T) {
	flows, v, _, b := openClip(t)
	b.ParsedFlow = &model.Flow{Nodes: []model.Node{{ID: "n", Type: "request", Request: &model.RequestCall{Request: model.Ref{Path: "orders/create"}}}}}

	pasted, err := flows.PasteSteps(v.Node, "text")

	if err != nil || len(pasted.IDs) != 1 || !slices.Equal(pasted.Unmatched, []string{"orders/create"}) {
		t.Fatalf("pasted %+v, err %v", pasted, err)
	}
}

func TestFlowService_CopyStepsPortable_Given_a_step_Then_it_is_copied_without_ids(t *testing.T) {
	flows, v, _, b := openClip(t)

	text, err := flows.CopyStepsPortable(v.Node, []string{"a"})

	if err != nil || text == "" || len(b.SourcedFlow.Nodes) != 1 {
		t.Fatalf("text %q, err %v, written %+v", text, err, b.SourcedFlow.Nodes)
	}
}
