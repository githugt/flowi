// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"cmp"
	"fmt"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/sites"
	"github.com/githugt/flowi-lib/internal/yamlpath"
	"github.com/githugt/flowi-lib/model"
)

// checkAttach checks that a node shown with a request (attach) can be drawn
// with it: the target is a request or subflow node of the same list, and the
// only nodes between the two are others attached to the same target. Anything
// else is a warning, not an error: attach changes the picture, not the run.
func checkAttach(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		for _, l := range sites.NodeLists(c) {
			for i, n := range l.Value {
				if detail := attachProblem(l.Value, i); detail != "" {
					at := diag.Location{File: l.File, Path: yamlpath.Key(yamlpath.Index(l.Path, i), "attach")}
					out = append(out, diag.New(diag.CodeNodeAttach, at, map[string]string{"node": n.ID, "target": cmp.Or(n.Attach, "nothing"), "detail": detail}))
				}
			}
		}
	}
	return out
}

// attachProblem says why node i of list cannot be drawn with its attach target,
// or "" when it can (or has none).
func attachProblem(list []model.Node, i int) string {
	n := list[i]
	if n.Attach == "" {
		if n.AttachAt != "" {
			return "attach_at needs attach: it says where around a request the node is drawn"
		}
		return ""
	}
	if n.AttachAt != "" && n.AttachAt != "above" && n.AttachAt != "below" {
		return fmt.Sprintf("attach_at %q must be above or below, or left out", n.AttachAt)
	}
	j := -1
	for k, m := range list {
		if m.ID == n.Attach {
			j = k
		}
	}
	switch {
	case j < 0:
		return "no node of that name is in the same list"
	case j == i || (list[j].Type != "request" && list[j].Type != "subflow"):
		return "it is not a request or subflow node"
	case n.Type == "request" || n.Type == "subflow":
		return "a request or subflow node is not attached to another"
	}
	lo, hi := min(i, j)+1, max(i, j)
	for k := lo; k < hi; k++ {
		if list[k].Attach != n.Attach {
			return fmt.Sprintf("%q sits between them", list[k].ID)
		}
	}
	return ""
}
