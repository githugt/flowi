// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/sites"
	"github.com/githugt/flowi/lib/internal/yamlpath"
	"github.com/githugt/flowi/lib/model"
)

// checkLoopControl requires every loop_control step to sit in the body of a batch or run_till, at any depth of
// conditions and parallels: it acts on the nearest loop around it, so a flow's own top level, a subflow's and a hook's
// inline steps are outside any loop.
func checkLoopControl(ix *index) diag.List {
	var out diag.List
	for _, c := range ix.collections {
		for _, root := range sites.RootLists(c) {
			out = append(out, loopControlProblems(root, false)...)
		}
	}
	return out
}

// loopControlProblems walks one list: inLoop says the list is inside a loop's body.
func loopControlProblems(l sites.NodeList, inLoop bool) diag.List {
	var out diag.List
	for i, n := range l.Value {
		at := yamlpath.Index(l.Path, i)
		if n.Type == "loop_control" && !inLoop {
			out = append(out, diag.New(diag.CodeNodeLoopControl, diag.Location{File: l.File, Path: at}, map[string]string{"node": n.ID}))
		}
		for _, child := range sites.ChildLists(n, at) {
			// a loop's body starts a loop; every other list (then, else, cases, branches) keeps the one around it
			inside := inLoop || (isLoop(n) && child.Path == yamlpath.Key(at, "body"))
			out = append(out, loopControlProblems(sites.NodeList{File: l.File, Path: child.Path, Value: child.Value}, inside)...)
		}
	}
	return out
}

func isLoop(n model.Node) bool { return n.Type == "batch" || n.Type == "run_till" }
