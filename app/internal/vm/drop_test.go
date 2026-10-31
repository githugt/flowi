// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func dropFixture(t *testing.T) (*vm.Workspace, *vmtest.Backend, *vm.Worker) {
	col := vmtest.SampleCollection()
	col.Folders = []model.Folder{{Dir: "requests/archive"}}
	b := &vmtest.Backend{Cols: []model.Collection{col}}
	ws, w := loadedWorkspace(t, b)
	return ws, b, w
}

func TestWorkspaceVM_DropOn_should_move_a_request_into_a_folder_or_to_the_top_of_a_collection(t *testing.T) {
	ws, b, w := dropFixture(t)
	health := ws.Node("r:C1:R3") // requests/health
	archive := ws.Node("c:C1/requests/archive")

	started := ws.DropOn(health, archive, nil, nil)
	w.Wait()
	login := ws.Node("r:C1:R1") // requests/auth/login
	top := ws.DropOn(login, ws.Node("c:C1"), nil, nil)
	w.Wait()

	if !started || !top || len(b.Calls) < 2 || !b.Called("MoveRequest") {
		t.Fatalf("started %v top %v calls %v", started, top, b.Calls)
	}
}

func TestWorkspaceVM_DropOn_should_ignore_a_drop_that_changes_nothing(t *testing.T) {
	ws, b, _ := dropFixture(t)
	login := ws.Node("r:C1:R1")

	onItsFolder := ws.DropOn(login, ws.Node("c:C1/requests/auth"), nil, nil)
	onARequest := ws.DropOn(login, ws.Node("r:C1:R3"), nil, nil)

	if onItsFolder || onARequest || len(b.Calls) > 1 {
		t.Fatalf("onItsFolder %v onARequest %v calls %v", onItsFolder, onARequest, b.Calls)
	}
}

func TestWorkspaceVM_DropOn_should_move_a_folder_but_never_into_itself(t *testing.T) {
	ws, b, w := dropFixture(t)
	auth := ws.Node("c:C1/requests/auth")

	into := ws.DropOn(auth, ws.Node("c:C1/requests/archive"), nil, nil)
	w.Wait()
	onSelf := ws.DropOn(auth, auth, nil, nil)

	if !into || onSelf || !b.Called("MoveFolder requests/auth -> archive/auth") {
		t.Fatalf("into %v onSelf %v calls %v", into, onSelf, b.Calls)
	}
}

func TestWorkspaceVM_DeleteFolder_and_ContentsOf_should_delete_and_count(t *testing.T) {
	ws, b, w := dropFixture(t)
	auth := ws.Node("c:C1/requests/auth")
	requests, flows := ws.ContentsOf(auth.ID)

	ws.DeleteFolder(auth, nil)
	w.Wait()

	if requests != 2 || flows != 0 || !b.Called("DeleteFolder requests/auth") {
		t.Fatalf("contents %d %d calls %v", requests, flows, b.Calls)
	}
}
