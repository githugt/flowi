// SPDX-License-Identifier: Apache-2.0

// Package sites enumerates where things live in loaded collections: root
// node lists, hook scopes and steps, template sites, written variables and
// statically resolved request versions. validate and analysis share it so
// they agree on what a template reads and where it is written.
package sites

import (
	"maps"
	"path"
	"strings"

	"github.com/githugt/flowi-lib/internal/yamlpath"
	"github.com/githugt/flowi-lib/model"
)

// Located is any value together with the file and YAML path it came from.
type Located[T any] struct {
	File  string
	Path  string
	Value T
}

// HookScopes lists every place hooks can be attached, with the YAML path
// of the mapping that holds the "hooks" key.
func HookScopes(c model.Collection) []Located[model.Hooks] {
	out := []Located[model.Hooks]{{c.Manifest.File, "$", c.Manifest.Hooks}}
	for _, f := range c.Folders {
		out = append(out, Located[model.Hooks]{f.File, "$", f.Hooks})
	}
	for _, r := range c.Requests {
		out = append(out, Located[model.Hooks]{r.File, "$", r.Hooks})
		for _, v := range r.Versions {
			out = append(out, Located[model.Hooks]{r.File, yamlpath.Key(yamlpath.Key("$", "versions"), v.Label), v.Hooks})
		}
	}
	return out
}

// HookSteps lists every hook step in the collection.
func HookSteps(c model.Collection) []Located[model.HookStep] {
	var out []Located[model.HookStep]
	for _, scope := range HookScopes(c) {
		hooks := yamlpath.Key(scope.Path, "hooks")
		for i, s := range scope.Value.Pre {
			out = append(out, Located[model.HookStep]{scope.File, yamlpath.Index(yamlpath.Key(hooks, "pre"), i), s})
		}
		for i, s := range scope.Value.Post {
			out = append(out, Located[model.HookStep]{scope.File, yamlpath.Index(yamlpath.Key(hooks, "post"), i), s})
		}
		for i, s := range scope.Value.Message {
			out = append(out, Located[model.HookStep]{scope.File, yamlpath.Index(yamlpath.Key(hooks, "message"), i), s})
		}
	}
	return out
}

// NodeList is a list of sibling nodes and the YAML path of the list.
type NodeList = Located[[]model.Node]

// RootLists returns the top-level node lists: each flow's nodes and each
// hook step's inline nodes. Node ids must be unique within one root.
func RootLists(c model.Collection) []NodeList {
	var roots []NodeList
	for _, f := range c.Flows {
		roots = append(roots, NodeList{f.File, yamlpath.Key("$", "nodes"), f.Nodes})
	}
	for _, s := range HookSteps(c) {
		if len(s.Value.Inline) > 0 {
			roots = append(roots, NodeList{s.File, yamlpath.Key(s.Path, "inline"), s.Value.Inline})
		}
	}
	return roots
}

// NodeLists returns every node list in the collection, top-level and
// nested inside containers.
func NodeLists(c model.Collection) []NodeList {
	var out []NodeList
	for _, r := range RootLists(c) {
		out = append(out, ExpandLists(r)...)
	}
	return out
}

// ExpandLists returns l and every node list nested inside its nodes.
func ExpandLists(l NodeList) []NodeList {
	out := []NodeList{l}
	for i, n := range l.Value {
		for _, child := range ChildLists(n, yamlpath.Index(l.Path, i)) {
			out = append(out, ExpandLists(NodeList{l.File, child.Path, child.Value})...)
		}
	}
	return out
}

// ChildLists returns the node lists directly inside n (body, branches,
// then, else, cases); p is n's YAML path. File is left empty.
func ChildLists(n model.Node, p string) []NodeList {
	var out []NodeList
	add := func(path string, nodes []model.Node) {
		if len(nodes) > 0 {
			out = append(out, NodeList{Path: path, Value: nodes})
		}
	}
	add(yamlpath.Key(p, "body"), n.Body)
	for i, b := range n.Branches {
		add(yamlpath.Key(yamlpath.Index(yamlpath.Key(p, "branches"), i), "nodes"), b.Nodes)
	}
	add(yamlpath.Key(p, "then"), n.Then)
	add(yamlpath.Key(p, "else"), n.Else)
	for i, cs := range n.Cases {
		add(yamlpath.Key(yamlpath.Index(yamlpath.Key(p, "cases"), i), "nodes"), cs.Nodes)
	}
	return out
}

// AllNodes flattens NodeLists into individual nodes with their paths.
func AllNodes(c model.Collection) []Located[model.Node] {
	var out []Located[model.Node]
	for _, l := range NodeLists(c) {
		for i, n := range l.Value {
			out = append(out, Located[model.Node]{l.File, yamlpath.Index(l.Path, i), n})
		}
	}
	return out
}

// AncestorFolders are the _folder.yaml folders on the way down to r.
func AncestorFolders(c model.Collection, r model.Request) []model.Folder {
	dir := path.Join("requests", path.Dir(r.Path))
	var out []model.Folder
	for _, f := range c.Folders {
		if dir == f.Dir || strings.HasPrefix(dir, f.Dir+"/") {
			out = append(out, f)
		}
	}
	return out
}

// VisibleDescendants are the nodes inside n readable after it: not those
// of repeated bodies (batch, run_till), which are read through results.
func VisibleDescendants(n model.Node) map[string]bool {
	out := map[string]bool{}
	if n.Type == "batch" || n.Type == "run_till" {
		return out
	}
	for _, list := range n.Children() {
		for _, c := range list {
			out[c.ID] = true
			maps.Copy(out, VisibleDescendants(c))
		}
	}
	return out
}
