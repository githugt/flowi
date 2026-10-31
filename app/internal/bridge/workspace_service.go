// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"path"

	"github.com/githugt/flowi/internal/vm"
)

// TreeNode is one row of the workspace tree.
type TreeNode struct {
	ID       string     `json:"id"`
	Label    string     `json:"label"`
	Kind     string     `json:"kind"` // collection, missing, requests, folder, request, environments, environment, flows, flow
	Method   string     `json:"method,omitempty"`
	Name     string     `json:"name,omitempty"` // what a folder's order calls it: the file or folder name
	Draft    bool       `json:"draft,omitempty"`
	Tree     string     `json:"tree,omitempty"` // folders and groups: requests or flows; requests can be dropped only on the first
	Children []TreeNode `json:"children,omitempty"`
}

// Tree is the workspace as the sidebar shows it.
type Tree struct {
	Name         string            `json:"name"`
	Folder       string            `json:"folder"` // where workspace.yaml lives; "" when unknown
	Roots        []TreeNode        `json:"roots"`
	Environments []string          `json:"environments"`
	EnvColours   map[string]string `json:"envColours"` // by environment name; absent: the default colour
	Selected     string            `json:"selected"`
}

func kindNames() map[vm.NodeKind]string {
	return map[vm.NodeKind]string{
		vm.NodeCollection:        "collection",
		vm.NodeMissing:           "missing",
		vm.NodeRequestsGroup:     "requests",
		vm.NodeFolder:            "folder",
		vm.NodeRequest:           "request",
		vm.NodeEnvironmentsGroup: "environments",
		vm.NodeEnvironment:       "environment",
		vm.NodeFlowsGroup:        "flows",
		vm.NodeFlow:              "flow",
	}
}

// WorkspaceService is the workspace tree and the selected environment.
type WorkspaceService struct{ app *App }

// Tree is the workspace now.
func (s *WorkspaceService) Tree() Tree {
	s.app.treeReady.wait() // an event sent before the window listens would be lost
	var t Tree
	s.app.actor.Do(func() { t = s.tree() })
	return t
}

// LocationView says where the collection of a tree node lives: the footer
// shows it for the tab in front.
type LocationView struct {
	Name    string `json:"name"`
	Path    string `json:"path"`   // the collection's folder; shown on hover
	Branch  string `json:"branch"` // the git branch checked out there, "" when the folder is in no repository
	Scratch bool   `json:"scratch"`
}

// Location is where the collection holding tree node `node` lives; the zero
// value when the node belongs to no collection.
func (s *WorkspaceService) Location(node string) LocationView {
	var loc LocationView
	s.app.actor.Do(func() {
		ws := s.app.workspace
		c, ok := ws.Collection(ws.Node(node).Collection)
		if !ok {
			return
		}
		loc = LocationView{Name: c.Manifest.Name, Path: c.Root, Scratch: ws.InScratch(node)}
		if !loc.Scratch {
			loc.Branch = vm.GitBranch(c.Root) // reads two small files; the tab changes seldom
		}
	})
	return loc
}

// entryName is what a folder's order calls a row: the last part of its path. Only
// requests, flows and folders can be ordered.
func entryName(n vm.TreeNode) string {
	switch n.Kind {
	case vm.NodeRequest, vm.NodeFlow, vm.NodeFolder:
		return path.Base(n.Path)
	}
	return ""
}

// ReorderAsk is sent as the "reorder-needs-confirm" event when putting entries in
// another order would rewrite a folder file that has comments: the window asks,
// then calls Reorder again with dropComments.
type ReorderAsk struct {
	Parent string   `json:"parent"`
	Names  []string `json:"names"`
}

// Reorder sets the order of the entries inside tree row `parent` (a collection's
// Requests or Flows row, or a folder) to names, the entries' names top to bottom.
// The tree changes when it is done; a problem is shown as a notice.
func (s *WorkspaceService) Reorder(parent string, names []string, dropComments bool) {
	s.app.actor.Do(func() {
		ws := s.app.workspace
		n := ws.Node(parent)
		dir := ""
		switch n.Kind {
		case vm.NodeRequestsGroup, vm.NodeFlowsGroup:
		case vm.NodeFolder:
			dir = n.Path
		default:
			s.app.emit("notice", Notice{Level: "error", Text: "only the entries of a folder can be put in another order"})
			return
		}
		ws.SetFolderOrder(n.Collection, n.Tree, dir, names, dropComments, func(err error) {
			if vm.IsDropsComments(err) {
				s.app.emit("reorder-needs-confirm", ReorderAsk{Parent: parent, Names: names})
				return
			}
			s.app.emit("notice", Notice{Level: "error", Text: vm.ErrorText(err)})
		})
	})
}

// SelectEnvironment chooses the environment requests and flows run with.
func (s *WorkspaceService) SelectEnvironment(name string) {
	s.app.actor.Do(func() { s.app.workspace.SelectEnv(name) })
}

// tree builds the Tree; call it on the actor.
func (s *WorkspaceService) tree() Tree {
	ws := s.app.workspace
	return Tree{
		Name:         ws.WorkspaceName(),
		Folder:       s.app.workspaceFolder(),
		Roots:        s.nodes(ws.Children("")),
		Environments: append([]string{}, ws.Environments()...),
		EnvColours:   ws.EnvColours(),
		Selected:     ws.SelectedEnv(),
	}
}

func (s *WorkspaceService) nodes(ids []string) []TreeNode {
	out := make([]TreeNode, 0, len(ids))
	for _, id := range ids {
		n := s.app.workspace.Node(id)
		out = append(out, TreeNode{
			ID: id, Label: s.app.workspace.Display(id), Kind: kindNames()[n.Kind],
			Method: n.Method, Name: entryName(n), Draft: n.Draft, Tree: n.Tree, Children: s.nodes(s.app.workspace.Children(id)),
		})
	}
	return out
}
