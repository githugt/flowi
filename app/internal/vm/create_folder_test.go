// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func TestWorkspaceVM_CreateFolder_should_show_an_empty_folder_in_the_tree(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}}
	ws, w := loadedWorkspace(t, b)

	ws.CreateFolder("C1", "requests", "billing", nil)
	w.Wait()

	n := ws.Node("c:C1/requests/billing")
	if n.Kind != vm.NodeFolder || n.Path != "billing" || n.Tree != "requests" || !b.Called("NewFolder requests/billing") {
		t.Fatalf("node %+v calls %v", n, b.CallLog())
	}
}

func TestMenuFor_folders_should_offer_a_new_folder_and_the_right_kind_of_item(t *testing.T) {
	col := vmtest.SampleCollection()
	col.Folders = []model.Folder{{Dir: "flows/nightly"}}
	b := &vmtest.Backend{Cols: []model.Collection{col}}
	ws, _ := loadedWorkspace(t, b)

	flowFolder := actionsOf(ws.MenuFor([]string{"c:C1/flows/nightly"}))
	requests := actionsOf(ws.MenuFor([]string{"c:C1/requests"}))

	if !slices.Equal(flowFolder, []vm.Action{vm.ActNewFlow, vm.ActNewFolder, vm.ActRename, vm.ActDelete}) || !slices.Contains(requests, vm.ActNewFolder) || !slices.Contains(requests, vm.ActNewRequest) {
		t.Fatalf("flow folder %v requests %v", flowFolder, requests)
	}
}

func TestWorkspaceVM_CreateFolder_Given_no_name_Then_makes_new_folder_inside_the_parent(t *testing.T) {
	tests := map[string]string{"": "requests/new-folder", "auth/": "requests/auth/new-folder", "auth/x": "requests/auth/x"}
	for given, want := range tests {
		b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}}
		ws, w := loadedWorkspace(t, b)

		ws.CreateFolder("C1", "requests", given, nil)
		w.Wait()

		if !b.Called("NewFolder " + want) {
			t.Errorf("name %q: calls %v, want NewFolder %s", given, b.CallLog(), want)
		}
	}
}

func TestWorkspaceVM_CreateFolder_Given_the_same_name_in_requests_and_flows_Then_each_tree_has_its_own_folder(t *testing.T) {
	// Before: nothing showed that a folder made under Flows stays out of Requests, and the other way round.
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}}
	ws, w := loadedWorkspace(t, b)

	ws.CreateFolder("C1", "requests", "billing", nil)
	ws.CreateFolder("C1", "flows", "billing", nil)
	w.Wait()

	inRequests := ws.Node("c:C1/requests/billing")
	inFlows := ws.Node("c:C1/flows/billing")
	if inRequests.Kind != vm.NodeFolder || inRequests.Tree != "requests" || inFlows.Kind != vm.NodeFolder || inFlows.Tree != "flows" || inRequests.ID == inFlows.ID {
		t.Fatalf("requests %+v flows %+v", inRequests, inFlows)
	}
}

func TestWorkspaceVM_DeleteFolder_Given_the_same_name_in_both_trees_Then_only_that_tree_is_asked_to_delete(t *testing.T) {
	col := vmtest.SampleCollection()
	col.Folders = []model.Folder{{Dir: "requests/billing"}, {Dir: "flows/billing"}}
	b := &vmtest.Backend{Cols: []model.Collection{col}}
	ws, w := loadedWorkspace(t, b)

	ws.DeleteFolder(ws.Node("c:C1/flows/billing"), nil)
	w.Wait()

	if !b.Called("DeleteFolder flows/billing") || b.Called("DeleteFolder requests/billing") {
		t.Fatalf("calls %v", b.CallLog())
	}
}
