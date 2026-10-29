// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"strconv"

	"github.com/githugt/flowi-lib/model"
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
			return false, from + " is in another lane than " + to + ": only one lane runs, or they run side by side"
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
