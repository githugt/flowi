// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"slices"
	"testing"

	flowi "github.com/githugt/flowi-lib"

	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
	"github.com/githugt/flowi/internal/vm/vmtest"
)

func shopWorkspace(t *testing.T) (*vm.Workspace, *vmtest.Backend, *vm.Worker) {
	t.Helper()
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}}
	ws, w := loadedWorkspace(t, b)
	return ws, b, w
}

func actionsOf(items []vm.MenuItem) []vm.Action {
	var out []vm.Action
	for _, i := range items {
		out = append(out, i.Action)
	}
	return out
}

func TestMenuFor_should_offer_what_a_single_row_can_do(t *testing.T) {
	ws, _, _ := shopWorkspace(t)
	tests := map[string]struct {
		id   string
		want []vm.Action
	}{
		"request":     {"r:C1:R1", []vm.Action{vm.ActOpen, vm.ActDuplicate, vm.ActRename, vm.ActDelete}},
		"flow":        {"f:C1:F1", []vm.Action{vm.ActOpen, vm.ActDuplicate, vm.ActCopyMermaid, vm.ActDelete}},
		"environment": {"e:/c/environments/local.yaml", []vm.Action{vm.ActOpen, vm.ActDelete}},
		"collection":  {"c:C1", []vm.Action{vm.ActNewRequest, vm.ActNewAsyncRequest, vm.ActNewGRPCRequest, vm.ActNewFlow, vm.ActNewFolder, vm.ActFlowFromRequests}},
		"folder":      {"c:C1/requests/auth", []vm.Action{vm.ActNewRequest, vm.ActNewAsyncRequest, vm.ActNewGRPCRequest, vm.ActNewFolder, vm.ActFlowFromRequests, vm.ActRename, vm.ActDelete}},
		"flows group": {"c:C1/flows", []vm.Action{vm.ActNewFlow, vm.ActNewFolder}},
		"envs group":  {"environments", []vm.Action{vm.ActNewEnvironment}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := actionsOf(ws.MenuFor([]string{tc.id}))

			if !slices.Equal(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMenuFor_Given_a_draft_collection_Then_offers_to_move_or_delete_it(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}, DraftIDs: map[model.ID]bool{"C1": true}}
	ws, _ := loadedWorkspace(t, b)

	got := actionsOf(ws.MenuFor([]string{"c:C1"}))

	if !slices.Contains(got, vm.ActMoveToFolder) || !slices.Contains(got, vm.ActDiscardDraft) {
		t.Fatalf("got %v", got)
	}
}

func TestMenuFor_Given_an_empty_collection_Then_no_flow_from_requests(t *testing.T) {
	c := vmtest.SampleCollection()
	c.Requests = nil
	ws, _ := loadedWorkspace(t, &vmtest.Backend{Cols: []model.Collection{c}})

	got := actionsOf(ws.MenuFor([]string{"c:C1"}))

	if slices.Contains(got, vm.ActFlowFromRequests) {
		t.Fatalf("got %v", got)
	}
}

func TestMenuFor_Given_several_requests_Then_flow_from_them_and_delete_with_counts(t *testing.T) {
	ws, _, _ := shopWorkspace(t)

	got := ws.MenuFor([]string{"r:C1:R1", "r:C1:R3"})

	want := []vm.MenuItem{{Action: vm.ActFlowFromRequests, Count: 2}, {Action: vm.ActDelete, Count: 2}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMenuFor_Given_mixed_rows_Then_only_delete(t *testing.T) {
	ws, _, _ := shopWorkspace(t)

	got := ws.MenuFor([]string{"r:C1:R1", "f:C1:F1"})

	if want := []vm.MenuItem{{Action: vm.ActDelete, Count: 2}}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMenuFor_Given_a_collection_among_several_Then_nothing(t *testing.T) {
	ws, _, _ := shopWorkspace(t)

	got := ws.MenuFor([]string{"c:C1", "r:C1:R1"})

	if len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestRequestsUnder_should_list_requests_in_tree_order(t *testing.T) {
	ws, _, _ := shopWorkspace(t)

	collection := ws.RequestsUnder("c:C1")
	folder := ws.RequestsUnder("c:C1/requests/auth")
	one := ws.RequestsUnder("r:C1:R3")

	if !slices.Equal(collection, []string{"r:C1:R1", "r:C1:R2", "r:C1:R3"}) || !slices.Equal(folder, []string{"r:C1:R1", "r:C1:R2"}) || !slices.Equal(one, []string{"r:C1:R3"}) {
		t.Fatalf("collection %v folder %v one %v", collection, folder, one)
	}
}

func TestWorkspaceVM_Delete_should_remove_the_items_and_report_their_nodes(t *testing.T) {
	ws, b, w := shopWorkspace(t)
	var gone []string

	ws.Delete([]string{"r:C1:R1", "f:C1:F1", "e:/c/environments/local.yaml"}, func(ids []string) { gone = ids }, nil)
	w.Wait()

	if !slices.Equal(gone, []string{"r:C1:R1", "f:C1:F1", "e:/c/environments/local.yaml"}) {
		t.Fatalf("gone %v calls %v", gone, b.Calls)
	}
	if ws.Node("r:C1:R1").Kind != 0 || ws.Node("f:C1:F1").Kind != 0 || ws.Node("e:/c/environments/local.yaml").Kind != 0 || ws.Node("r:C1:R2").Kind == 0 {
		t.Fatal("the tree must show the deletion and nothing else")
	}
}

func TestWorkspaceVM_Delete_Given_a_failure_Then_reports_the_ones_gone_and_stops(t *testing.T) {
	ws, b, w := shopWorkspace(t)
	b.FailDeleteAt = 2
	var failed error
	var gone []string

	ws.Delete([]string{"r:C1:R1", "r:C1:R2", "r:C1:R3"}, func(ids []string) { gone = ids }, func(err error) { failed = err })
	w.Wait()

	if failed == nil || !slices.Equal(gone, []string{"r:C1:R1"}) || b.Called("DeleteRequest R3") {
		t.Fatalf("failed %v gone %v calls %v", failed, gone, b.Calls)
	}
}

func TestWorkspaceVM_DuplicateRequest_should_copy_then_open_the_copy(t *testing.T) {
	ws, b, w := shopWorkspace(t)
	var opened string

	ws.DuplicateRequest(flowi.RequestTarget{Collection: "C1", Request: "R3"}, "health-copy", func(id string) { opened = id }, nil)
	w.Wait()

	if opened != "r:C1:R3-copy" || !b.Called("DuplicateRequest health-copy") || ws.Node("r:C1:R3-copy").Kind != vm.NodeRequest {
		t.Fatalf("opened %q calls %v", opened, b.Calls)
	}
}

func TestWorkspaceVM_DuplicateFlow_should_copy_then_open_the_copy(t *testing.T) {
	ws, _, w := shopWorkspace(t)
	var opened string

	ws.DuplicateFlow(flowi.FlowTarget{Collection: "C1", Flow: "F1"}, "signup-copy", func(id string) { opened = id }, nil)
	w.Wait()

	if opened != "f:C1:F1-copy" || ws.Node(opened).Kind != vm.NodeFlow {
		t.Fatalf("opened %q", opened)
	}
}

func TestWorkspaceVM_FlowFromRequests_should_chain_one_request_node_per_request(t *testing.T) {
	ws, b, w := shopWorkspace(t)
	var opened string

	ws.FlowFromRequests("C1", "journeys/all", []string{"r:C1:R1", "r:C1:R2", "r:C1:R3"}, func(id string) { opened = id }, nil)
	w.Wait()

	if len(b.SavedFlows) != 1 || opened != "f:C1:new" {
		t.Fatalf("saved %d opened %q calls %v", len(b.SavedFlows), opened, b.Calls)
	}
	nodes := b.SavedFlows[0].Nodes
	if len(nodes) != 3 {
		t.Fatalf("nodes %+v", nodes)
	}
	if nodes[0].Request.Request.ID != "R1" || nodes[0].Request.Request.Path != "auth/login" || len(nodes[0].Needs) != 0 {
		t.Fatalf("first %+v", nodes[0])
	}
	// Behaviour change: the nodes used to be written needing the one before. Nodes
	// run in list order and a node without needs depends on the previous one by
	// itself (design §7.2), so those needs said nothing and showed up in the
	// inspector as dependencies the requests do not have.
	if nodes[2].Request.Request.ID != "R3" || len(nodes[1].Needs) != 0 || len(nodes[2].Needs) != 0 || nodes[1].ID == nodes[2].ID {
		t.Fatalf("chain %+v", nodes)
	}
}

func TestWorkspaceVM_FlowMermaid_should_return_the_saved_flow_as_a_chart(t *testing.T) {
	ws, _, w := shopWorkspace(t)
	var chart string

	ws.FlowMermaid(flowi.FlowTarget{Collection: "C1", Flow: "F1"}, func(s string) { chart = s }, nil)
	w.Wait()

	if chart == "" {
		t.Fatal("no chart")
	}
}

// Marks are made in any order; the flow reads the requests top to bottom.
func TestWorkspaceVM_FlowFromRequests_should_chain_in_tree_order_not_mark_order(t *testing.T) {
	ws, b, w := shopWorkspace(t)

	ws.FlowFromRequests("C1", "j/x", []string{"r:C1:R3", "r:C1:R1"}, nil, nil)
	w.Wait()

	nodes := b.SavedFlows[0].Nodes
	if len(nodes) != 2 || nodes[0].Request.Request.ID != "R1" || nodes[1].Request.Request.ID != "R3" {
		t.Fatalf("nodes %+v", nodes)
	}
}

func TestWorkspaceVM_FlowFromRequests_Given_a_failed_save_Then_removes_the_empty_flow_and_reports(t *testing.T) {
	ws, b, w := shopWorkspace(t)
	b.SaveErr = errBoom
	var failed error

	ws.FlowFromRequests("C1", "j/x", []string{"r:C1:R1"}, nil, func(err error) { failed = err })
	w.Wait()

	if failed == nil || !b.Called("DeleteFlow new") {
		t.Fatalf("failed %v calls %v", failed, b.Calls)
	}
}

func TestWorkspaceVM_FlowFromRequests_Given_no_requests_Then_fails_without_creating_anything(t *testing.T) {
	ws, b, w := shopWorkspace(t)
	var failed error

	ws.FlowFromRequests("C1", "j/x", []string{"f:C1:F1"}, nil, func(err error) { failed = err })
	w.Wait()

	if failed == nil || b.Called("NewFlow j/x") {
		t.Fatalf("failed %v calls %v", failed, b.Calls)
	}
}

func TestWorkspaceVM_FlowFromRequests_Given_no_path_Then_makes_an_untitled_flow_without_one(t *testing.T) {
	ws, b, w := shopWorkspace(t)
	var opened string

	ws.FlowFromRequests("C1", "", []string{"r:C1:R1", "r:C1:R2"}, func(id string) { opened = id }, nil)
	w.Wait()

	if !b.Called("NewUntitledFlow C1") || b.Called("NewFlow ") || len(b.SavedFlows) != 1 || len(b.SavedFlows[0].Nodes) != 2 || opened != "f:C1:new" {
		t.Fatalf("calls %v saved %+v opened %q", b.Calls, b.SavedFlows, opened)
	}
}

func TestWorkspace_RenameFolder_should_move_the_folder_through_the_backend(t *testing.T) {
	ws, b, w := shopWorkspace(t)
	folder := ws.Node("c:C1/requests/auth")

	ws.RenameFolder(folder, "security", func(err error) { t.Fatal(err) })
	w.Wait()

	if !b.Called("MoveFolder requests/auth -> security") {
		t.Fatalf("calls %v", b.Calls)
	}
}
