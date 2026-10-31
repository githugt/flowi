// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"errors"
	"path"
	"slices"
	"strings"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"
)

// Action is something the tree's context menu can do to the rows it was
// opened on.
type Action int

// Context menu actions.
const (
	ActOpen Action = iota + 1
	ActDuplicate
	ActRename
	ActDelete
	ActCopyMermaid
	ActNewRequest
	ActNewEnvironment
	ActNewFlow
	ActFlowFromRequests
	ActMoveToFolder
	ActDiscardDraft
	ActManage
	ActNewFolder
	ActNewAsyncRequest
	ActNewGRPCRequest
	ActNewDatasetRequest
)

// MenuItem is one entry of the context menu. Count is how many rows or
// requests the action covers, for the entries that say so ("Delete 3…").
type MenuItem struct {
	Action Action
	Count  int
}

// MenuFor is the context menu for the given rows (one row, or the marked
// rows). Several rows only offer what applies to all of them: a flow from
// requests of one collection, and deleting leaves.
func (ws *Workspace) MenuFor(ids []string) []MenuItem {
	nodes := ws.nodesOf(ids)
	switch len(nodes) {
	case 0:
		return nil
	case 1:
		return ws.menuForOne(nodes[0])
	}
	return ws.menuForMany(nodes)
}

func (ws *Workspace) nodesOf(ids []string) []TreeNode {
	var out []TreeNode
	for _, id := range ids {
		if n := ws.Node(id); n.Kind != 0 {
			out = append(out, n)
		}
	}
	return out
}

func (ws *Workspace) menuForOne(n TreeNode) []MenuItem {
	switch n.Kind {
	case NodeRequest:
		return items(ActOpen, ActDuplicate, ActRename, ActDelete)
	case NodeFlow:
		return items(ActOpen, ActDuplicate, ActCopyMermaid, ActDelete)
	case NodeEnvironment:
		return items(ActOpen, ActDelete)
	case NodeFlowsGroup:
		return items(ActNewFlow, ActNewFolder)
	case NodeEnvironmentsGroup:
		return items(ActNewEnvironment)
	case NodeMissing:
		return items(ActManage)
	case NodeFolder, NodeRequestsGroup:
		if n.Tree == "flows" {
			return withDelete(n, items(ActNewFlow, ActNewFolder))
		}
		return withDelete(n, append(items(ActNewRequest, ActNewAsyncRequest, ActNewGRPCRequest, ActNewDatasetRequest, ActNewFolder), ws.flowFromRequestsItem(n.ID)...))
	case NodeCollection:
		out := append(items(ActNewRequest, ActNewAsyncRequest, ActNewGRPCRequest, ActNewDatasetRequest, ActNewFlow, ActNewFolder), ws.flowFromRequestsItem(n.ID)...)
		if n.Draft {
			out = append(out, items(ActMoveToFolder, ActDiscardDraft)...)
		}
		return out
	}
	return nil
}

func (ws *Workspace) flowFromRequestsItem(id string) []MenuItem {
	if n := len(ws.RequestsUnder(id)); n > 0 {
		return []MenuItem{{Action: ActFlowFromRequests, Count: n}}
	}
	return nil
}

// withDelete adds Rename / move and Delete to a folder's menu (the groups
// can be neither renamed nor deleted).
func withDelete(n TreeNode, menu []MenuItem) []MenuItem {
	if n.Kind == NodeFolder {
		return append(menu, MenuItem{Action: ActRename}, MenuItem{Action: ActDelete})
	}
	return menu
}

func (ws *Workspace) menuForMany(nodes []TreeNode) []MenuItem {
	var out []MenuItem
	if allRequestsOfOneCollection(nodes) {
		out = append(out, MenuItem{Action: ActFlowFromRequests, Count: len(nodes)})
	}
	if slices.ContainsFunc(nodes, func(n TreeNode) bool { return !isLeaf(n) }) {
		return out
	}
	return append(out, MenuItem{Action: ActDelete, Count: len(nodes)})
}

func isLeaf(n TreeNode) bool {
	return n.Kind == NodeRequest || n.Kind == NodeFlow || n.Kind == NodeEnvironment
}

func allRequestsOfOneCollection(nodes []TreeNode) bool {
	return !slices.ContainsFunc(nodes, func(n TreeNode) bool {
		return n.Kind != NodeRequest || n.Collection != nodes[0].Collection
	})
}

func items(actions ...Action) []MenuItem {
	out := make([]MenuItem, len(actions))
	for i, a := range actions {
		out[i] = MenuItem{Action: a}
	}
	return out
}

// RequestsUnder lists the request nodes at or below id, in tree order.
func (ws *Workspace) RequestsUnder(id string) []string {
	if ws.Node(id).Kind == NodeRequest {
		return []string{id}
	}
	var out []string
	for _, child := range ws.Children(id) {
		out = append(out, ws.RequestsUnder(child)...)
	}
	return out
}

// ContentsOf counts the requests and flows at any depth below row id (a
// folder), for asking before it is deleted.
func (ws *Workspace) ContentsOf(id string) (requests, flows int) {
	for _, child := range ws.Children(id) {
		switch n := ws.Node(child); n.Kind {
		case NodeRequest:
			requests++
		case NodeFlow:
			flows++
		default:
			r, f := ws.ContentsOf(child)
			requests, flows = requests+r, flows+f
		}
	}
	return requests, flows
}

// DeleteFolder removes folder row n and everything in it, then shows the
// tree without it.
func (ws *Workspace) DeleteFolder(n TreeNode, fail func(error)) {
	ws.create(func() (string, error) {
		return "", ws.b.DeleteFolder(n.Collection, n.Tree, n.Path)
	}, nil, fail)
}

// RenameFolder moves folder row n to the path to (its new place under the
// same tree), with everything in it, then shows the tree with it there.
func (ws *Workspace) RenameFolder(n TreeNode, to string, fail func(error)) {
	ws.create(func() (string, error) { return "", ws.b.MoveFolder(n.Collection, n.Tree, n.Path, to) }, nil, fail)
}

// DropOn is what dropping row dragged on row target does: a request or a
// folder of requests moves into the folder, or to the top of the collection,
// that target is. It reports whether anything was started. Moves of a
// folder stay inside its collection; a request may change collection.
func (ws *Workspace) DropOn(dragged, target TreeNode, open func(nodeID string), fail func(error)) bool {
	dest, ok := dropPath(dragged, target)
	if !ok {
		return false
	}
	if dragged.Kind == NodeRequest {
		ws.MoveRequest(flowi.RequestTarget{Collection: dragged.Collection, Request: dragged.Request}, target.Collection, dest, open, fail)
		return true
	}
	ws.create(func() (string, error) { return "", ws.b.MoveFolder(dragged.Collection, "requests", dragged.Path, dest) }, nil, fail)
	return true
}

// CanDrop reports whether DropOn would do anything for these rows.
func (ws *Workspace) CanDrop(dragged, target TreeNode) bool {
	_, ok := dropPath(dragged, target)
	return ok
}

// dropPath is the new path of dragged when dropped on target, or false when
// the drop changes nothing or is not allowed.
func dropPath(dragged, target TreeNode) (string, bool) {
	into, ok := dropDestination(target)
	if !ok || dragged.ID == target.ID {
		return "", false
	}
	switch dragged.Kind {
	case NodeRequest:
		dest := joinPath(into, path.Base(dragged.Path))
		return dest, target.Collection != dragged.Collection || dest != dragged.Path
	case NodeFolder:
		if dragged.Tree != "requests" || target.Collection != dragged.Collection {
			return "", false
		}
		dest := joinPath(into, path.Base(dragged.Path))
		inItself := into == dragged.Path || strings.HasPrefix(into+"/", dragged.Path+"/")
		return dest, dest != dragged.Path && !inItself
	}
	return "", false
}

// dropDestination is the folder (path under requests; "" for the top) that
// dropping on target means, or false when target takes nothing.
func dropDestination(target TreeNode) (string, bool) {
	switch target.Kind {
	case NodeFolder:
		return target.Path, target.Tree == "requests"
	case NodeRequestsGroup, NodeCollection:
		return "", true
	}
	return "", false
}

func joinPath(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}

// Delete removes the rows' items (requests, flows, environments), then
// shows the tree without them and calls removed with the node ids that
// went. It stops at the first failure: removed gets those already gone and
// fail the error. Other kinds of rows are ignored.
func (ws *Workspace) Delete(ids []string, removed func(nodeIDs []string), fail func(error)) {
	var nodes []TreeNode
	for _, n := range ws.nodesOf(ids) {
		if isLeaf(n) {
			nodes = append(nodes, n)
		}
	}
	type outcome struct {
		gone []string
		err  error
		snap snapshot
	}
	Run(ws.w, func() outcome {
		var o outcome
		for _, n := range nodes {
			if o.err = ws.deleteNode(n); o.err != nil {
				break
			}
			o.gone = append(o.gone, n.ID)
		}
		o.snap = buildSnapshot(ws.b)
		return o
	}, func(o outcome) {
		ws.apply(o.snap)
		if len(o.gone) > 0 && removed != nil {
			removed(o.gone)
		}
		if o.err != nil && fail != nil {
			fail(o.err)
		}
	})
}

// deleteNode runs off the UI goroutine.
func (ws *Workspace) deleteNode(n TreeNode) error {
	switch n.Kind {
	case NodeRequest:
		return ws.b.DeleteRequest(flowi.RequestTarget{Collection: n.Collection, Request: n.Request})
	case NodeFlow:
		return ws.b.DeleteFlow(flowi.FlowTarget{Collection: n.Collection, Flow: n.Flow})
	case NodeEnvironment:
		return ws.b.DeleteEnvironment(n.File)
	}
	return nil
}

// DuplicateRequest copies request t to path, then calls open with the
// copy's node id.
func (ws *Workspace) DuplicateRequest(t flowi.RequestTarget, path string, open func(nodeID string), fail func(error)) {
	ws.create(func() (string, error) {
		copied, err := ws.b.DuplicateRequest(t, path)
		return requestNodeID(copied), err
	}, open, fail)
}

// DuplicateRequestTo copies request t to path in collection to (which may be
// its own), then calls open with the copy's node id. The original is untouched.
func (ws *Workspace) DuplicateRequestTo(t flowi.RequestTarget, to model.ID, path string, open func(nodeID string), fail func(error)) {
	ws.create(func() (string, error) {
		copied, err := ws.b.DuplicateRequestTo(t, to, path)
		return requestNodeID(copied), err
	}, open, fail)
}

// DuplicateFlow copies flow t to path, then calls open with the copy's node
// id.
func (ws *Workspace) DuplicateFlow(t flowi.FlowTarget, path string, open func(nodeID string), fail func(error)) {
	ws.create(func() (string, error) {
		copied, err := ws.b.DuplicateFlow(t, path)
		return "f:" + string(copied.Collection) + ":" + string(copied.Flow), err
	}, open, fail)
}

// FlowFromRequests creates a flow at path in collection ("" means Scratch;
// an empty path is the next free "untitled") with one request node for each of the request rows, in order, and opens it. Requests of another collection are left out:
// their references would need a dependency.
func (ws *Workspace) FlowFromRequests(collection model.ID, path string, requestNodes []string, open func(nodeID string), fail func(error)) {
	requests := ws.requestsInTreeOrder(collection, requestNodes)
	ws.create(func() (string, error) {
		if len(requests) == 0 {
			return "", errNoRequests
		}
		var t flowi.FlowTarget
		var err error
		name := "Flow from requests"
		if path == "" {
			t, err = ws.b.NewUntitledFlow(requests[0].Collection)
		} else {
			t, err = ws.b.NewFlow(requests[0].Collection, path)
			name = pathBase(path)
		}
		if err != nil {
			return "", err
		}
		f := model.Flow{Meta: model.Meta{Name: name}, Nodes: chainOf(requests)}
		if err := ws.b.SaveFlow(t, f, false); err != nil {
			// Do not leave the empty flow behind; the save error is the one to show.
			_ = ws.b.DeleteFlow(t)
			return "", err
		}
		return "f:" + string(t.Collection) + ":" + string(t.Flow), nil
	}, open, fail)
}

var errNoRequests = errors.New("there are no requests to make a flow from")

// requestsInTreeOrder is the request rows among ids that belong to
// collection (the first request's, when it is empty), in the order the tree
// lists them: marks are made in any order, a flow reads top to bottom.
func (ws *Workspace) requestsInTreeOrder(collection model.ID, ids []string) []TreeNode {
	var out []TreeNode
	for _, id := range ws.RequestsUnder("") {
		if !slices.Contains(ids, id) {
			continue
		}
		n := ws.Node(id)
		if collection == "" {
			collection = n.Collection
		}
		if n.Collection == collection {
			out = append(out, n)
		}
	}
	return out
}

func pathBase(p string) string { return path.Base(p) }

// chainOf is a request node for each request, in order. The nodes carry no
// needs: a node without needs depends on the one before it by itself.
func chainOf(requests []TreeNode) []model.Node {
	taken := map[string]bool{}
	var nodes []model.Node
	for _, r := range requests {
		n := model.Node{
			ID:      freeID(path.Base(r.Path), taken),
			Type:    "request",
			Request: &model.RequestCall{Request: model.Ref{ID: r.Request, Path: r.Path}},
		}
		taken[n.ID] = true
		nodes = append(nodes, n)
	}
	return nodes
}

// FlowMermaid hands the saved flow t, as a Mermaid flowchart, to done.
func (ws *Workspace) FlowMermaid(t flowi.FlowTarget, done func(chart string), fail func(error)) {
	type result struct {
		chart string
		err   error
	}
	Run(ws.w, func() result {
		chart, err := ws.b.ExportMermaid(t)
		return result{chart, err}
	}, func(r result) {
		switch {
		case r.err != nil && fail != nil:
			fail(r.err)
		case r.err == nil && done != nil:
			done(r.chart)
		}
	})
}
