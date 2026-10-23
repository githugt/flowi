// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"path"
	"slices"
	"strings"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
)

// Requests could only be renamed by typing a new path, inside their own
// collection, one at a time, and the entry was called "Rename / move" though
// the dialog asked for nothing but the path. Moving is now its own action:
// any number of requests, to any collection or request folder, chosen in a
// list or by dropping them on a row of the tree.

// MoveTargetView is a place requests can be moved to: a collection's
// requests, or a folder among them.
type MoveTargetView struct {
	Row   string `json:"row"`   // the tree row: a collection or a request folder
	Label string `json:"label"` // "Payments / auth / tokens"
	Depth int    `json:"depth"` // 0 for a collection
	Draft bool   `json:"draft,omitempty"`
}

// renamedPath is where a renamed row goes: name in the same folder. A name
// with a slash is a path from the top, as the old Rename / move took.
func renamedPath(old, name string) string {
	if name == "" || strings.Contains(name, "/") {
		return strings.Trim(name, "/")
	}
	if dir := path.Dir(old); dir != "." {
		return dir + "/" + name
	}
	return name
}

// withMove adds Move to… to the menu of one or more requests: after Rename
// for one, before Delete for several.
func withMove(entries []MenuEntry, ws *vm.Workspace, ids []string) []MenuEntry {
	if len(ids) == 0 || slices.ContainsFunc(ids, func(id string) bool { return ws.Node(id).Kind != vm.NodeRequest }) {
		return entries
	}
	// No "dirty" guard: a move takes the edit slot along, and the window stores open edits first (Keep).
	move := MenuEntry{Action: "move", Label: "Move to…", Dialog: "move"}
	at := slices.IndexFunc(entries, func(e MenuEntry) bool { return e.Action == "rename" }) + 1
	if at == 0 {
		at = slices.IndexFunc(entries, func(e MenuEntry) bool { return e.Action == "delete" })
	}
	if at < 0 {
		at = len(entries)
	}
	return slices.Insert(entries, at, move)
}

// MoveTargets lists where requests can go: every collection, its request
// folders under it, in tree order.
func (s *WorkspaceService) MoveTargets() []MoveTargetView {
	out := []MoveTargetView{}
	s.app.actor.Do(func() {
		ws := s.app.workspace
		for _, id := range ws.Children("") {
			n := ws.Node(id)
			if n.Kind != vm.NodeCollection {
				continue
			}
			out = append(out, MoveTargetView{Row: id, Label: ws.Display(id), Draft: n.Draft})
			out = append(out, requestFolders(ws, id, ws.Display(id), 1)...)
		}
	})
	return out
}

// requestFolders are the request folders below row, depth first.
func requestFolders(ws *vm.Workspace, row, label string, depth int) []MoveTargetView {
	var out []MoveTargetView
	for _, id := range ws.Children(row) {
		n := ws.Node(id)
		switch {
		case n.Kind == vm.NodeRequestsGroup:
			out = append(out, requestFolders(ws, id, label, depth)...)
		case n.Kind == vm.NodeFolder && n.Tree == "requests":
			name := label + " / " + path.Base(n.Path)
			out = append(out, MoveTargetView{Row: id, Label: name, Depth: depth})
			out = append(out, requestFolders(ws, id, name, depth+1)...)
		}
	}
	return out
}

// moveDestination is the collection and folder a row stands for: a
// collection or its requests group is the top of its requests, a request
// folder is itself. ok is false for any other row.
func moveDestination(ws *vm.Workspace, row string) (collection model.ID, folder string, ok bool) {
	n := ws.Node(row)
	switch {
	case n.Kind == vm.NodeCollection, n.Kind == vm.NodeRequestsGroup:
		return n.Collection, "", true
	case n.Kind == vm.NodeFolder && n.Tree == "requests":
		return n.Collection, n.Path, true
	}
	return "", "", false
}

// MoveInto moves requests into a collection or request folder (a row of the
// tree), each keeping its name. A request already there stays; one whose
// name is taken there is reported and stays. A single moved request opens
// again at its new place.
func (s *WorkspaceService) MoveInto(ids []string, row string) {
	s.app.actor.Do(func() {
		ws, emit := s.app.workspace, s.app.emit
		collection, folder, ok := moveDestination(ws, row)
		if !ok {
			emit("notice", Notice{Level: "error", Text: "Requests can be moved into a collection or a request folder."})
			return
		}
		fail := func(err error) { emit("notice", Notice{Level: "error", Text: vm.ErrorText(err)}) }
		var open func(string)
		if len(ids) == 1 {
			open = func(node string) { emit("open", node) }
		}
		for _, id := range ids {
			n := ws.Node(id)
			if n.Kind != vm.NodeRequest {
				continue
			}
			to := path.Join(folder, path.Base(n.Path))
			if n.Collection == collection && to == n.Path {
				continue
			}
			ws.MoveRequest(flowi.RequestTarget{Collection: n.Collection, Request: n.Request}, collection, to, open, fail)
		}
	})
}
