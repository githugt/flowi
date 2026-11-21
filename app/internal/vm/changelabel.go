// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/githugt/flowi/lib/model"
)

// DescribeChange says in a few words what changed between two versions of a flow: "add step login",
// "delete 3 steps", "edit step login", "edit 2 steps", "edit the flow's settings". It is the text of the Undo and
// Redo buttons.
//
// Before: Undo and Redo showed only on or off, so nobody could see what the click would take back.
func DescribeChange(from, to model.Flow) string {
	a, b := indexSteps(from.Nodes), indexSteps(to.Nodes)
	var added, removed, edited []string
	for id, n := range b {
		if _, ok := a[id]; !ok {
			added = append(added, stepName(n))
		}
	}
	for id, n := range a {
		if _, ok := b[id]; !ok {
			removed = append(removed, stepName(n))
		}
	}
	changed := map[string]bool{}
	for id, n := range b {
		if old, ok := a[id]; ok && !reflect.DeepEqual(old, n) {
			changed[id] = true
		}
	}
	for id := range changed {
		if !hasChangedInside(b[id], changed) {
			edited = append(edited, stepName(b[id]))
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(edited)

	settings := false
	fa, fb := from, to
	fa.Nodes, fb.Nodes = nil, nil
	settings = !reflect.DeepEqual(fa, fb)

	kinds := 0
	for _, l := range [][]string{added, removed, edited} {
		if len(l) > 0 {
			kinds++
		}
	}
	if settings {
		kinds++
	}
	switch {
	case kinds == 0:
		return "change"
	case kinds > 1:
		return "change the flow"
	case len(added) > 0:
		return "add " + stepsText(added)
	case len(removed) > 0:
		return "delete " + stepsText(removed)
	case len(edited) > 0:
		return "edit " + stepsText(edited)
	}
	return "edit the flow's settings"
}

// stepsText is "step login" for one and "3 steps" for more.
func stepsText(names []string) string {
	if len(names) == 1 {
		return "step " + names[0]
	}
	return fmt.Sprintf("%d steps", len(names))
}

func stepName(n model.Node) string {
	if n.Name != "" {
		return n.Name
	}
	return n.ID
}

// indexSteps maps the id of every step, those inside containers too, to the step.
func indexSteps(list []model.Node) map[string]model.Node {
	out := map[string]model.Node{}
	var walk func([]model.Node)
	walk = func(l []model.Node) {
		for _, n := range l {
			out[n.ID] = n
			for _, c := range n.Children() {
				walk(c)
			}
		}
	}
	walk(list)
	return out
}

// hasChangedInside says whether a step inside n is one of the changed ones: then n changed only because of it.
func hasChangedInside(n model.Node, changed map[string]bool) bool {
	for _, c := range n.Children() {
		for _, k := range c {
			if changed[k.ID] || hasChangedInside(k, changed) {
				return true
			}
		}
	}
	return false
}

// UndoLabel is what Undo would take back, as "Undo: delete 3 steps"; "Undo" when there is nothing to undo.
func (d *FlowDoc) UndoLabel() string {
	if len(d.undo) == 0 {
		return "Undo"
	}
	return "Undo: " + DescribeChange(d.undo[len(d.undo)-1], d.draft)
}

// RedoLabel is what Redo would bring back, as "Redo: delete 3 steps".
func (d *FlowDoc) RedoLabel() string {
	if len(d.redo) == 0 {
		return "Redo"
	}
	return "Redo: " + DescribeChange(d.draft, d.redo[len(d.redo)-1])
}
