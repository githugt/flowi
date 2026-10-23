// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"maps"
	"slices"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/sites"
	"github.com/githugt/flowi-lib/internal/yamlpath"
	"github.com/githugt/flowi-lib/model"
)

// checkConcurrentWrites rejects two nodes that may run at the same time
// writing the same target (design §7.3): nodes in different branches of a
// parallel, and siblings where the later one is concurrent and does not
// depend on the earlier one.
func checkConcurrentWrites(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		for _, l := range sites.NodeLists(c) {
			out = append(out, siblingClashes(l)...)
			for i, n := range l.Value {
				out = append(out, branchClashes(l, i, n)...)
			}
		}
	}
	return out
}

// siblingClashes: j concurrent and i not among j's dependencies.
func siblingClashes(l sites.NodeList) diag.List {
	var out diag.List
	for j, b := range l.Value {
		if !b.Concurrent {
			continue
		}
		ancestors := dependencyClosure(l.Value, j)
		for i, a := range l.Value[:j] {
			if ancestors[i] {
				continue
			}
			if target, clash := sharedTarget(writesOf(a), writesOf(b)); clash {
				out = append(out, clashDiag(l, j, a.ID, b.ID, target))
			}
		}
	}
	return out
}

// branchClashes: any two branches of one parallel.
func branchClashes(l sites.NodeList, i int, n model.Node) diag.List {
	var out diag.List
	for x := range n.Branches {
		for y := x + 1; y < len(n.Branches); y++ {
			a, b := listWrites(n.Branches[x].Nodes), listWrites(n.Branches[y].Nodes)
			if target, clash := sharedTarget(a, b); clash {
				out = append(out, clashDiag(l, i, a[target], b[target], target))
			}
		}
	}
	return out
}

func clashDiag(l sites.NodeList, at int, a, b, target string) diag.Diagnostic {
	loc := diag.Location{File: l.File, Path: yamlpath.Index(l.Path, at)}
	return diag.New(diag.CodeNodeWriteClash, loc, map[string]string{"a": a, "b": b, "target": target})
}

// sharedTarget returns the first target (in name order) both write.
func sharedTarget(a, b map[string]string) (string, bool) {
	for _, t := range slices.Sorted(maps.Keys(a)) {
		if _, ok := b[t]; ok {
			return t, true
		}
	}
	return "", false
}

// dependencyClosure marks every sibling node j transitively depends on.
func dependencyClosure(list []model.Node, j int) map[int]bool {
	index := map[string]int{}
	for i, n := range list {
		index[n.ID] = i
	}
	seen := map[int]bool{}
	var visit func(k int)
	visit = func(k int) {
		deps := list[k].Needs
		if len(deps) == 0 && k > 0 {
			deps = []string{list[k-1].ID}
		}
		for _, d := range deps {
			if i, ok := index[d]; ok && !seen[i] {
				seen[i] = true
				visit(i)
			}
		}
	}
	visit(j)
	return seen
}

// listWrites maps each target the nodes write to the first writer's id.
func listWrites(list []model.Node) map[string]string {
	out := map[string]string{}
	for _, n := range list {
		for t, id := range writesOf(n) {
			if _, seen := out[t]; !seen {
				out[t] = id
			}
		}
	}
	return out
}

// writesOf maps every target a node (with its non-isolated descendants)
// writes to the writing node's id. Batch bodies write to iteration-local
// variables; subflows and request nodes have their own.
func writesOf(n model.Node) map[string]string {
	out := map[string]string{}
	for _, t := range sites.OwnWrites(n) {
		out[t] = n.ID
	}
	if n.Type == "batch" {
		return out
	}
	for _, list := range n.Children() {
		for t, id := range listWrites(list) {
			if _, seen := out[t]; !seen {
				out[t] = id
			}
		}
	}
	return out
}
