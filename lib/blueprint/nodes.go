// SPDX-License-Identifier: Apache-2.0

package blueprint

import (
	"slices"

	"github.com/githugt/flowi-lib/model"
)

// cloneNodes copies a node tree and calls edit on every copy, children
// first. The original is never changed: lists, calls and branches are
// copied, while opaque field values (Params) are shared, because nothing
// edits them in place.
func cloneNodes(nodes []model.Node, edit func(*model.Node)) []model.Node {
	if nodes == nil {
		return nil
	}
	out := make([]model.Node, len(nodes))
	for i, n := range nodes {
		out[i] = cloneNode(n, edit)
	}
	return out
}

func cloneNode(n model.Node, edit func(*model.Node)) model.Node {
	n.Needs = slices.Clone(n.Needs)
	n.Params = slices.Clone(n.Params)
	if n.Request != nil {
		c := *n.Request
		c.Hooks.Skip = slices.Clone(c.Hooks.Skip)
		c.With, c.Headers = slices.Clone(c.With), slices.Clone(c.Headers)
		n.Request = &c
	}
	if n.Subflow != nil {
		c := *n.Subflow
		c.With = slices.Clone(c.With)
		n.Subflow = &c
	}
	n.Body = cloneNodes(n.Body, edit)
	n.Then, n.Else = cloneNodes(n.Then, edit), cloneNodes(n.Else, edit)
	if n.Branches != nil {
		n.Branches = slices.Clone(n.Branches)
		for i := range n.Branches {
			n.Branches[i].Nodes = cloneNodes(n.Branches[i].Nodes, edit)
		}
	}
	if n.Cases != nil {
		n.Cases = slices.Clone(n.Cases)
		for i := range n.Cases {
			n.Cases[i].Nodes = cloneNodes(n.Cases[i].Nodes, edit)
		}
	}
	edit(&n)
	return n
}

// walkNodes calls visit for every node in the tree, in document order.
func walkNodes(nodes []model.Node, visit func(model.Node)) {
	for _, n := range nodes {
		visit(n)
		for _, list := range n.Children() {
			walkNodes(list, visit)
		}
	}
}
