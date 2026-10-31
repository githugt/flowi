// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"errors"
	"testing"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

// Behaviour change: a collection stored in Flowi's own folder is an
// ordinary collection (the user's wording); only its Draft flag tells the
// manager where it lives. It used to be labelled "(draft)".
func TestWorkspaceVM_should_mark_collections_in_flowis_folder_without_a_draft_label(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}, DraftIDs: map[model.ID]bool{"C1": true}}

	ws, _ := loadedWorkspace(t, b)

	root := ws.Node(ws.Children("")[0])
	if root.Label != "Shop" || !root.Draft {
		t.Fatalf("root %+v", root)
	}
}

func TestWorkspaceVM_NewScratchRequest_should_create_reload_and_open(t *testing.T) {
	b := &vmtest.Backend{}
	ws, w := loadedWorkspace(t, b)
	var opened string

	ws.NewScratchRequest(func(id string) { opened = id }, nil)
	w.Wait()

	if !b.Called("NewScratchRequest") || opened != "r:SCRATCH:new" {
		t.Fatalf("calls %v, opened %q", b.Calls, opened)
	}
}

func TestWorkspaceVM_CreateRequest_Given_no_collection_Then_uses_Scratch(t *testing.T) {
	b := &vmtest.Backend{}
	ws, w := loadedWorkspace(t, b)
	var opened string

	ws.CreateRequest("", "users/list", func(id string) { opened = id }, nil)
	w.Wait()

	if !b.Called("Scratch") || !b.Called("NewRequest users/list") || opened != "r:SCRATCH:new" {
		t.Fatalf("calls %v, opened %q", b.Calls, opened)
	}
}

func TestWorkspaceVM_CreateFlow_Given_no_collection_Then_uses_Scratch(t *testing.T) {
	b := &vmtest.Backend{}
	ws, w := loadedWorkspace(t, b)
	var opened string

	ws.CreateFlow("", "signup", func(id string) { opened = id }, nil)
	w.Wait()

	if !b.Called("Scratch") || !b.Called("NewFlow signup") || opened != "f:SCRATCH:new" {
		t.Fatalf("calls %v, opened %q", b.Calls, opened)
	}
}

func TestWorkspaceVM_CreateRequest_Given_backend_error_Then_reports_it(t *testing.T) {
	b := &vmtest.Backend{NewErr: errBoom}
	ws, w := loadedWorkspace(t, b)
	var got error

	ws.CreateRequest("C1", "x", func(string) { t.Fatal("opened") }, func(err error) { got = err })
	w.Wait()

	if got == nil {
		t.Fatal("no error")
	}
}

func TestWorkspaceVM_CreateRequest_Given_a_collection_Then_seeds_its_hooks(t *testing.T) {
	b := &vmtest.Backend{}
	ws, w := loadedWorkspace(t, b)

	ws.CreateRequest("C1", "users/list", func(string) {}, nil)
	w.Wait()

	if !b.Called("SeedRequestHooks new") {
		t.Fatalf("calls %v", b.Calls)
	}
}

// Before: a request made in Scratch had no hooks, and moves with them
// when it is saved into a collection only if they are its own.
func TestWorkspaceVM_NewScratchRequest_Then_seeds_its_hooks(t *testing.T) {
	b := &vmtest.Backend{}
	ws, w := loadedWorkspace(t, b)

	ws.NewScratchRequest(func(string) {}, nil)
	w.Wait()

	if !b.Called("SeedRequestHooks new") {
		t.Fatalf("calls %v", b.Calls)
	}
}

func TestWorkspaceVM_CreateRequest_Given_seeding_fails_Then_the_request_opens_and_the_error_is_reported(t *testing.T) {
	b := &vmtest.Backend{SeedErr: errBoom}
	ws, w := loadedWorkspace(t, b)
	var opened string
	var got error

	ws.CreateRequest("C1", "x", func(id string) { opened = id }, func(err error) { got = err })
	w.Wait()

	if opened == "" || got == nil {
		t.Fatalf("opened %q, err %v", opened, got)
	}
}

func TestManagerVM_drafts_rows_and_actions(t *testing.T) {
	m, b, w := newManager(t)
	b.Mu.Lock()
	b.DraftIDs = map[model.ID]bool{"C2": true}
	b.Mu.Unlock()
	m.Load()
	w.Wait()

	m.NewDraft("Try")
	m.SaveDraft("C2", "/keep")
	m.Discard("C2")
	w.Wait()

	if !rowByID(m.Rows(), "C2").Draft || rowByID(m.Rows(), "C1").Draft {
		t.Fatalf("rows %+v", m.Rows())
	}
	for _, call := range []string{"NewDraft Try", "SaveDraft C2 /keep", "DiscardDraft C2"} {
		if !b.Called(call) {
			t.Fatalf("missing %q in %v", call, b.Calls)
		}
	}
}

func rowByID(rows []vm.CollectionRow, id model.ID) vm.CollectionRow {
	for _, r := range rows {
		if r.ID == id {
			return r
		}
	}
	return vm.CollectionRow{}
}

var errBoom = errors.New("boom")

func TestWorkspaceVM_MoveRequest_should_move_then_open_the_new_node(t *testing.T) {
	b := draftsBackend()
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	ws.Load()
	w.Wait()
	var opened string

	ws.MoveRequest(flowi.RequestTarget{Collection: "S1", Request: "SR1"}, "C1", "users/signin", func(id string) { opened = id }, nil)
	w.Wait()

	if opened != "r:C1:SR1" || len(b.Moves) != 1 || b.Moves[0] != [2]string{"S1/SR1", "C1/users/signin"} || ws.Node("r:C1:SR1").Kind != vm.NodeRequest {
		t.Fatalf("opened %q moves %v", opened, b.Moves)
	}
	if ws.Node("r:S1:SR1").Kind != 0 {
		t.Fatal("the old node must be gone from the tree")
	}
}

func TestWorkspaceVM_CreateEnvironment_should_create_it_and_open_its_node(t *testing.T) {
	b := &vmtest.Backend{}
	ws, w := loadedWorkspace(t, b)
	var opened string

	ws.CreateEnvironment("staging", func(id string) { opened = id }, nil)
	w.Wait()

	if !b.Called("NewEnvironment staging") || opened != "e:/d/staging.yaml" {
		t.Fatalf("calls %v, opened %q", b.Calls, opened)
	}
}

func TestWorkspaceVM_CreateUntitledFlow_should_create_without_a_path_in_the_collection_or_in_Scratch(t *testing.T) {
	b := &vmtest.Backend{}
	ws, w := loadedWorkspace(t, b)
	var opened []string

	ws.CreateUntitledFlow("C1", func(id string) { opened = append(opened, id) }, nil)
	ws.CreateUntitledFlow("", func(id string) { opened = append(opened, id) }, nil)
	w.Wait()

	if !b.Called("NewUntitledFlow C1") || !b.Called("Scratch") || len(opened) != 2 || opened[0] != "f:C1:new" || opened[1] != "f:SCRATCH:new" {
		t.Fatalf("calls %v, opened %v", b.Calls, opened)
	}
}

// Behaviour change: "New flow" on a folder row used to create the flow at the
// top of the collection's flows, ignoring the folder that was clicked.
func TestWorkspaceVM_CreateUntitledFlowIn_should_create_inside_the_folder(t *testing.T) {
	b := &vmtest.Backend{}
	ws, w := loadedWorkspace(t, b)

	ws.CreateUntitledFlowIn("C1", "shared", func(string) {}, nil)
	ws.CreateUntitledFlowIn("C1", "", func(string) {}, nil)
	w.Wait()

	if !b.Called("NewFlow shared/untitled") || !b.Called("NewUntitledFlow C1") {
		t.Fatalf("calls %v", b.Calls)
	}
}
