// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"github.com/githugt/flowi-lib/model"
)

// RunScope says which part of a flow a partial run covers, around one step.
type RunScope int

const (
	// ScopeOnly runs the step alone, reading the others' outputs from the last run.
	ScopeOnly RunScope = iota
	// ScopeUntil runs the step and everything it depends on.
	ScopeUntil
	// ScopeFrom runs the step and everything that depends on it, reading the
	// others' outputs from the last run.
	ScopeFrom
)

// PartialNodes are the top-level nodes a partial run around id covers, in flow
// order. A step inside a condition, a batch or a strip stands for the top-level
// node that holds it. ok is false when id is not in the flow.
func PartialNodes(list []model.Node, id string, kind RunScope) (ids []string, ok bool) {
	at := topLevelIndex(list, id)
	if at < 0 {
		return nil, false
	}
	deps := dependencyIndexes(list)
	chosen := map[int]bool{at: true}
	switch kind {
	case ScopeUntil:
		addAncestors(chosen, deps, at)
	case ScopeFrom:
		addDescendants(chosen, deps)
	case ScopeOnly:
	}
	for i, n := range list {
		if chosen[i] {
			ids = append(ids, n.ID)
		}
	}
	return ids, true
}

// topLevelIndex is the index of the top-level node that is, or holds, id.
func topLevelIndex(list []model.Node, id string) int {
	for i := range list {
		if _, _, found := locate(list[i:i+1], "$", id); found {
			return i
		}
	}
	return -1
}

// dependencyIndexes mirrors the engine: a node waits for its needs, or for the
// node before it when it has none.
func dependencyIndexes(list []model.Node) [][]int {
	index := make(map[string]int, len(list))
	for i, n := range list {
		index[n.ID] = i
	}
	out := make([][]int, len(list))
	for i, n := range list {
		for _, need := range n.Needs {
			if j, ok := index[need]; ok {
				out[i] = append(out[i], j)
			}
		}
		if len(n.Needs) == 0 && i > 0 {
			out[i] = []int{i - 1}
		}
	}
	return out
}

func addAncestors(chosen map[int]bool, deps [][]int, i int) {
	for _, j := range deps[i] {
		if !chosen[j] {
			chosen[j] = true
			addAncestors(chosen, deps, j)
		}
	}
}

func addDescendants(chosen map[int]bool, deps [][]int) {
	for i := range deps {
		if chosen[i] {
			continue
		}
		for _, j := range deps[i] {
			if chosen[j] {
				chosen[i] = true
				addDescendants(chosen, deps)
				break
			}
		}
	}
}

// nodeIDsWithin is id and the ids of the steps inside the top-level node id.
func nodeIDsWithin(list []model.Node, id string) []string {
	ids := map[string]bool{}
	for _, n := range list {
		if n.ID == id {
			allIDs([]model.Node{n}, ids)
		}
	}
	out := make([]string, 0, len(ids))
	for inner := range ids {
		out = append(out, inner)
	}
	return out
}
