// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"strconv"

	"github.com/githugt/flowi/lib/model"
)

// hop is one step on the way down to a node: the list it is in and its place there.
type hop struct {
	list  string
	index int
}

// ancestry is the path from the top-level list down to node id: one hop for each
// list on the way, the last being the node itself.
func ancestry(list []model.Node, path, id string) ([]hop, bool) {
	for i, n := range list {
		if n.ID == id {
			return []hop{{list: path, index: i}}, true
		}
		for _, c := range childPaths(n, path+"["+strconv.Itoa(i)+"]") {
			if rest, ok := ancestry(c.nodes, c.path, id); ok {
				return append([]hop{{list: path, index: i}}, rest...), true
			}
		}
	}
	return nil, false
}

// CanRead says whether step `to` may read what step `from` makes, and if not,
// why. A step reads the steps before it in its own list and in the lists that
// enclose it (a step inside a loop reads what ran before the loop), and what a
// container made once it is done. It does not read a step that runs after it,
// one in another lane of a container (only one lane runs, or they run
// together), or the container it is inside.
func (d *FlowDoc) CanRead(from, to string) (bool, string) {
	a, okA := ancestry(d.draft.Nodes, "$.nodes", from)
	b, okB := ancestry(d.draft.Nodes, "$.nodes", to)
	switch {
	case !okA:
		return false, "there is no step " + from
	case !okB:
		return false, "there is no step " + to
	case from == to:
		return false, to + " cannot read itself"
	}
	for k := 0; k < len(a) && k < len(b); k++ {
		switch {
		case a[k].list != b[k].list:
			return false, from + " and " + to + " are in different branches: only one branch runs, or they run side by side, so " + to + " cannot use " + from + "'s result"
		case a[k].index < b[k].index:
			return true, ""
		case a[k].index > b[k].index:
			return false, from + " runs after " + to
		}
	}
	if len(a) > len(b) {
		return false, from + " is inside " + to + ", which has not finished yet"
	}
	return false, to + " is inside " + from + ", which has not finished yet"
}

// CanWait says whether step `to` may wait for step `from` (an arrow from `from` into `to`), and if not, why. A step
// waits only for a step of its own list that comes before it: the layout draws no arrow between lists (a step in a
// lane, a branch or a loop body and one outside it), and an arrow from a later step would be a cycle.
//
// Before: a link dropped on any step was accepted, and one the layout could not draw was saved and never shown.
func (d *FlowDoc) CanWait(from, to string) (bool, string) {
	fromList, fromAt, okFrom := d.Locate(from)
	toList, toAt, okTo := d.Locate(to)
	switch {
	case !okFrom:
		return false, "there is no step " + from
	case !okTo:
		return false, "there is no step " + to
	case from == to:
		return false, to + " cannot wait for itself"
	case fromList != toList:
		return false, from + " and " + to + " are in different lists (a lane, a branch or a loop body): a step waits only for a step next to it"
	case fromAt > toAt:
		return false, from + " comes after " + to + ", so " + to + " cannot wait for it: start the link from the earlier step"
	}
	return true, ""
}

// WireTarget says whether a wire drawn from a step may end on step ID, and why not when it may not.
type WireTarget struct {
	ID     string
	OK     bool
	Reason string
}

// Wire kinds for WireTargets.
const (
	WireWait = "wait" // an arrow: the target runs after the source (a port dragged to a step)
	WireData = "data" // a value: the target reads what the source made
)

// WireTargets answers, for every step of the flow, whether a wire of the given kind from step `from` may end there.
func (d *FlowDoc) WireTargets(from, kind string) []WireTarget {
	var out []WireTarget
	var visit func(list []model.Node, path string)
	visit = func(list []model.Node, path string) {
		for i, n := range list {
			var ok bool
			var why string
			if kind == WireData {
				ok, why = d.CanRead(from, n.ID)
			} else {
				ok, why = d.CanWait(from, n.ID)
			}
			out = append(out, WireTarget{ID: n.ID, OK: ok, Reason: why})
			for _, c := range childPaths(n, path+"["+strconv.Itoa(i)+"]") {
				visit(c.nodes, c.path)
			}
		}
	}
	visit(d.draft.Nodes, "$.nodes")
	return out
}
