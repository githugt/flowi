// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"fmt"
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/sites"
	"github.com/githugt/flowi-lib/internal/yamlpath"
	"github.com/githugt/flowi-lib/model"
)

// findCycles returns each distinct cycle in a directed graph once, rotated
// to start at its smallest vertex. Vertices are visited in sorted order so
// results are deterministic.
func findCycles(edges map[string][]string) [][]string {
	vertices := make([]string, 0, len(edges))
	for v := range edges {
		vertices = append(vertices, v)
	}
	slices.Sort(vertices)
	const (
		unvisited = iota
		onStack
		done
	)
	state := map[string]int{}
	var stack []string
	seen := map[string]bool{}
	var cycles [][]string
	var visit func(v string)
	visit = func(v string) {
		state[v] = onStack
		stack = append(stack, v)
		for _, next := range edges[v] {
			switch state[next] {
			case unvisited:
				visit(next)
			case onStack:
				cycle := rotateToMin(stack[slices.Index(stack, next):])
				if key := strings.Join(cycle, "\x00"); !seen[key] {
					seen[key] = true
					cycles = append(cycles, cycle)
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[v] = done
	}
	for _, v := range vertices {
		if state[v] == unvisited {
			visit(v)
		}
	}
	return cycles
}

func rotateToMin(cycle []string) []string {
	i := slices.Index(cycle, slices.Min(cycle))
	return append(slices.Clone(cycle[i:]), cycle[:i]...)
}

// describeCycle renders "a → b → a".
func describeCycle(names []string) string {
	return strings.Join(append(slices.Clone(names), names[0]), " → ")
}

// checkNeedsCycles finds dependency cycles within each node list. A node
// without needs depends on the previous sibling (serial by default).
func checkNeedsCycles(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		for _, l := range sites.NodeLists(c) {
			out = append(out, needsCycles(l)...)
		}
	}
	return out
}

func needsCycles(l sites.NodeList) diag.List {
	position := map[string]int{}
	for i, n := range l.Value {
		position[n.ID] = i
	}
	edges := map[string][]string{}
	for i, n := range l.Value {
		edges[n.ID] = effectiveNeeds(l.Value, i, position)
	}
	var out diag.List
	for _, cycle := range findCycles(edges) {
		loc := diag.Location{File: l.File, Path: yamlpath.Index(l.Path, position[cycle[0]])}
		out = append(out, diag.New(diag.CodeCycleNeeds, loc, map[string]string{"cycle": describeCycle(cycle)}))
	}
	return out
}

func effectiveNeeds(nodes []model.Node, i int, position map[string]int) []string {
	n := nodes[i]
	if len(n.Needs) == 0 {
		if i == 0 {
			return nil
		}
		return []string{nodes[i-1].ID}
	}
	var deps []string
	for _, target := range n.Needs {
		if _, sibling := position[target]; sibling { // non-siblings are reported by checkNodes
			deps = append(deps, target)
		}
	}
	return deps
}

// flowVertex orders flows by collection position, then path, so cycles
// start at the first flow a user would see.
type flowVertex struct {
	collection int
	path       string
	key        entityKey
}

func checkSubflowCycles(ix *index) diag.List {
	byKey := map[entityKey]flowVertex{}
	byName := map[string]flowVertex{}
	edges := map[string][]string{}
	for ci, c := range ix.collections {
		for _, f := range c.Flows {
			v := flowVertex{ci, f.Path, entityKey{c.Manifest.ID, f.ID}}
			byKey[v.key], byName[v.name()] = v, v
			edges[v.name()] = nil
		}
	}
	for _, c := range ix.collections {
		for _, f := range c.Flows {
			from := byKey[entityKey{c.Manifest.ID, f.ID}].name()
			for _, target := range ix.subflowTargets(c, f) {
				if v, ok := byKey[target]; ok {
					edges[from] = append(edges[from], v.name())
				}
			}
		}
	}
	var out diag.List
	for _, cycle := range findCycles(edges) {
		start := byName[cycle[0]]
		labels := make([]string, 0, len(cycle))
		for _, n := range cycle {
			labels = append(labels, ix.flowLabel(byName[n], start.collection))
		}
		loc := diag.Location{File: ix.flows[start.key].File, Path: "$"}
		out = append(out, diag.New(diag.CodeCycleSubflow, loc, map[string]string{"cycle": describeCycle(labels)}))
	}
	return out
}

// name is a sortable vertex key: zero-padded collection position, path, id.
func (v flowVertex) name() string {
	return fmt.Sprintf("%06d\x00%s\x00%s", v.collection, v.path, v.key.id)
}

// flowLabel names a flow by path, prefixed with its collection's name when
// it lives in a different collection from where the cycle is reported.
func (ix *index) flowLabel(v flowVertex, home int) string {
	if v.collection == home {
		return v.path
	}
	return ix.collections[v.collection].Manifest.Name + ":" + v.path
}

// subflowTargets lists the flows f calls through subflow nodes at any depth.
func (ix *index) subflowTargets(c model.Collection, f model.Flow) []entityKey {
	var out []entityKey
	var walk func(nodes []model.Node)
	walk = func(nodes []model.Node) {
		for _, n := range nodes {
			if n.Subflow != nil {
				if target, code, _ := ix.targetCollection(c, n.Subflow.Flow.Collection); code == "" {
					out = append(out, entityKey{target, n.Subflow.Flow.ID})
				}
			}
			for _, child := range n.Children() {
				walk(child)
			}
		}
	}
	walk(f.Nodes)
	return out
}
