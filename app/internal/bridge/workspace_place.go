// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"
	"path"
	"strings"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
)

// The chooser every "new" and "save as" dialog shares: a tree of collections and the folders
// inside them (the Move dialog's list), with a new collection or folder made in the dialog itself.
// An empty row means Scratch, the place a request goes when none is chosen.

// PlaceTargets lists where something of tree ("requests" or "flows") can go: every collection
// and the folders of that tree below it, in tree order.
func (s *WorkspaceService) PlaceTargets(tree string) []MoveTargetView {
	out := []MoveTargetView{}
	s.app.actor.Do(func() {
		ws := s.app.workspace
		for _, id := range ws.Children("") {
			n := ws.Node(id)
			if n.Kind != vm.NodeCollection {
				continue
			}
			out = append(out, MoveTargetView{Row: id, Label: ws.Display(id), Draft: n.Draft})
			out = append(out, treeFolders(ws, id, ws.Display(id), 1, tree)...)
		}
	})
	return out
}

// NewAt creates a request (kind request, async, grpc or dataset) or a flow at the place row stands for
// (a collection or a folder; "" is Scratch) and asks the window to open it. text is a name or a
// path below the place; blank makes the next "untitled". protocol is for async, and the database for a dataset query.
func (s *WorkspaceService) NewAt(kind, row, text, protocol string) {
	s.app.actor.Do(func() {
		ws, emit := s.app.workspace, s.app.emit
		tree := "requests"
		if kind == "flow" {
			tree = "flows"
		}
		collection, folder, ok := destinationOrScratch(ws, row, tree)
		if !ok {
			emit("notice", Notice{Level: "error", Text: "Choose a collection or a folder."})
			return
		}
		open := func(node string) { emit("open", node) }
		fail := func(err error) { emit("notice", Notice{Level: "error", Text: vm.ErrorText(err)}) }
		below := placePath(folder, text)
		switch kind {
		case "request":
			ws.CreateRequest(collection, below, open, fail)
		case "async":
			ws.CreateAsyncRequest(collection, below, protocol, open, fail)
		case "grpc":
			ws.CreateGRPCRequest(collection, below, open, fail)
		case "dataset":
			ws.CreateDatasetRequest(collection, below, protocol, open, fail)
		case "flow":
			if strings.TrimSpace(text) == "" {
				ws.CreateUntitledFlowIn(collection, folder, open, fail)
				return
			}
			ws.CreateFlow(collection, below, open, fail)
		default:
			fail(fmt.Errorf("unknown kind %q", kind))
		}
	})
}

// destinationOrScratch is placeDestination, but a blank row is Scratch: no collection, no folder.
func destinationOrScratch(ws *vm.Workspace, row, tree string) (collection model.ID, folder string, ok bool) {
	if row == "" {
		return "", "", true
	}
	return placeDestination(ws, row, tree)
}

// placePath is where text goes below folder: a blank text is the folder itself, so the
// caller numbers the "untitled" there.
func placePath(folder, text string) string {
	text = strings.Trim(strings.TrimSpace(text), "/")
	switch {
	case text == "" && folder == "":
		return ""
	case text == "":
		return folder + "/"
	}
	return path.Join(folder, text)
}

// NewPlaceCollection makes a draft collection named name and returns its row, "" with a
// notice when it could not be made. A draft is kept in Flowi's folder until saved to one of the
// person's own.
func (s *WorkspaceService) NewPlaceCollection(name string) string {
	return s.created(func(open func(string), fail func(error)) {
		s.app.workspace.CreateDraftCollection(name, open, fail)
	})
}

// NewPlaceFolder makes the folder text below the place row stands for, in tree, and returns the
// folder's row, "" with a notice when it could not be made.
func (s *WorkspaceService) NewPlaceFolder(row, tree, text string) string {
	return s.created(func(open func(string), fail func(error)) {
		collection, folder, ok := destinationOrScratch(s.app.workspace, row, tree)
		if !ok || collection == "" {
			fail(fmt.Errorf("choose a collection or a folder to make the folder in"))
			return
		}
		s.app.workspace.CreateFolderAt(collection, tree, placePath(folder, text), open, fail)
	})
}

// created runs start on the actor and waits for the row it opens. Waiting is outside the actor,
// which the creation needs to finish.
func (s *WorkspaceService) created(start func(open func(string), fail func(error))) string {
	rows := make(chan string, 1)
	s.app.actor.Do(func() {
		start(
			func(row string) { rows <- row },
			func(err error) {
				s.app.emit("notice", Notice{Level: "error", Text: vm.ErrorText(err)})
				rows <- ""
			})
	})
	return <-rows
}
