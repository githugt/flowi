// SPDX-License-Identifier: Apache-2.0

package mermaid

import (
	"regexp"
	"slices"
)

// subgraphHead reads the id of "subgraph id [title]"; without a bracketed title the
// whole text is both id and title.
var subgraphHead = regexp.MustCompile(`^(\S+)\s*\[`)

func (g *graph) openSubgraph(text string) {
	g.subgraphs = append(g.subgraphs, text)
	id := text
	if m := subgraphHead.FindStringSubmatch(text); m != nil {
		id = m[1]
	}
	g.addToOpen(id)
	g.open = append(g.open, id)
	if _, ok := g.members[id]; !ok {
		g.members[id] = nil
	}
}

func (g *graph) closeSubgraph() {
	if len(g.open) > 0 {
		g.open = g.open[:len(g.open)-1]
	}
}

// addToOpen records id as written inside every subgraph being read.
func (g *graph) addToOpen(id string) {
	for _, sg := range g.open {
		if sg != id && !slices.Contains(g.members[sg], id) {
			g.members[sg] = append(g.members[sg], id)
		}
	}
}

// resolveSubgraphs replaces arrows to or from a subgraph with arrows to its entry
// nodes (those nothing inside points at) or from its exit nodes, and drops the
// subgraph's own node: a subgraph groups nodes, it is not a step.
func (g *graph) resolveSubgraphs() {
	if len(g.members) == 0 {
		return
	}
	var edges []edge
	for _, e := range g.edges {
		for _, from := range g.ends(e.from, false) {
			for _, to := range g.ends(e.to, true) {
				edges = append(edges, edge{from: from, to: to})
			}
		}
	}
	g.edges = edges
	g.nodes = slices.DeleteFunc(g.nodes, func(n *gnode) bool {
		_, isSubgraph := g.members[n.id]
		delete(g.byID, n.id)
		return isSubgraph
	})
	for _, n := range g.nodes {
		g.byID[n.id] = n
	}
}

// ends is id itself for a node, and for a subgraph the nodes an arrow should reach
// (entries) or leave from (exits), looking through nested subgraphs.
func (g *graph) ends(id string, entries bool) []string {
	members, isSubgraph := g.members[id]
	if !isSubgraph {
		return []string{id}
	}
	var leaves []string
	for _, m := range members {
		leaves = append(leaves, g.leaves(m)...)
	}
	return slices.DeleteFunc(leaves, func(l string) bool { return g.linkedInside(l, leaves, entries) })
}

// leaves lists the nodes under id: id itself unless it is a subgraph.
func (g *graph) leaves(id string) []string {
	members, isSubgraph := g.members[id]
	if !isSubgraph {
		return []string{id}
	}
	var out []string
	for _, m := range members {
		out = append(out, g.leaves(m)...)
	}
	return out
}

// linkedInside reports whether another of group's nodes has an arrow into id (for
// entries) or out of it, which makes id an inner node rather than an end.
func (g *graph) linkedInside(id string, group []string, entries bool) bool {
	for _, e := range g.edges {
		near, far := e.to, e.from
		if !entries {
			near, far = e.from, e.to
		}
		if near == id && far != id && slices.Contains(group, far) {
			return true
		}
	}
	return false
}
