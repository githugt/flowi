// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"errors"
	"slices"
	"strings"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// TopSteps are the ids of the steps in the flow's own list, in order: what select-all selects. The steps inside a
// container go with the container.
func (d *FlowDoc) TopSteps() []string {
	out := make([]string, 0, len(d.draft.Nodes))
	for _, n := range d.draft.Nodes {
		out = append(out, n.ID)
	}
	return out
}

// CopySteps returns copies of the steps ids, in flow order. A step inside another selected step is not copied twice:
// it comes with its container. The arrows between the copies stay; an arrow to a step outside them is dropped, since it
// would point at nothing wherever they are put.
//
// Before: the canvas had no step copy and paste, so reusing a few steps meant adding each again by hand.
func (d *FlowDoc) CopySteps(ids []string) ([]model.Node, error) {
	if len(ids) == 0 {
		return nil, errors.New("select the steps to copy first")
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var out []model.Node
	var walk func(list []model.Node)
	walk = func(list []model.Node) {
		for i := range list {
			if want[list[i].ID] {
				out = append(out, cloneNodes(list[i:i+1])...)
				continue
			}
			for _, c := range list[i].Children() {
				walk(c)
			}
		}
	}
	walk(d.draft.Nodes)
	if len(out) == 0 {
		return nil, errors.New("those steps are not in the flow any more")
	}
	keep := map[string]bool{}
	allIDs(out, keep)
	dropOutsideArrows(out, keep)
	return out, nil
}

// PasteSteps puts copies of nodes, which came from the clipboard (this flow or another), after the step `after`, in
// its list; with no step to follow they go at the end of the flow's own list. Each gets a free id (references among
// them follow the new ids), and an arrow to a step this flow does not have is dropped. It returns the ids of the
// pasted steps, outermost only, and is one undo step.
func (d *FlowDoc) PasteSteps(nodes []model.Node, after string) ([]string, error) {
	if len(nodes) == 0 {
		return nil, errors.New("there are no steps to paste")
	}
	var made []string
	err := d.command(func(f *model.Flow) error {
		taken := map[string]bool{}
		allIDs(f.Nodes, taken)
		fresh := freshNodes(nodes, taken) // adds the new ids to taken
		dropOutsideArrows(fresh, taken)
		list, at := &f.Nodes, len(f.Nodes)
		if after != "" {
			path, i, ok := locate(f.Nodes, "$.nodes", after)
			if !ok {
				return errors.New("no step " + after + " to paste after")
			}
			l, _ := listAt(f, path)
			list, at = l, i+1
		}
		*list = slices.Insert(*list, at, fresh...)
		for _, n := range fresh {
			made = append(made, n.ID)
		}
		return nil
	})
	return made, err
}

// dropOutsideArrows removes, in list and everything inside it, the arrows (waits, failure waits, outcome waits) to a
// step that is not in keep, and a drawing attachment to one.
func dropOutsideArrows(list []model.Node, keep map[string]bool) {
	for i := range list {
		n := &list[i]
		n.Needs = slices.DeleteFunc(n.Needs, func(s string) bool { return !keep[s] })
		n.AfterFailureOf = slices.DeleteFunc(n.AfterFailureOf, func(s string) bool { return !keep[s] })
		n.AfterOutcome = slices.DeleteFunc(n.AfterOutcome, func(e string) bool {
			step, _, _ := strings.Cut(e, ":")
			return !keep[step]
		})
		if n.Attach != "" && !keep[n.Attach] {
			n.Attach, n.AttachAt = "", ""
		}
		for _, child := range listsOf(n) {
			dropOutsideArrows(*child, keep)
		}
	}
}

// CopyText is the steps ids as the text a flow's code view shows, for the clipboard: only those steps, so it reads as
// a small flow. The text is made off the UI goroutine.
func (d *FlowDoc) CopyText(ids []string, done func(text string), fail func(error)) {
	nodes, err := d.CopySteps(ids)
	if err != nil {
		fail(err)
		return
	}
	t := d.target
	type result struct {
		text flowi.FlowText
		err  error
	}
	Run(d.w, func() result {
		text, err := d.b.FlowSource(t, model.Flow{Nodes: nodes})
		return result{text, err}
	}, func(r result) {
		if r.err != nil {
			fail(r.err)
			return
		}
		done(r.text.Flow)
	})
}

// PasteText reads text as copied steps and puts them in after step `after` (the end when it is empty or not found).
// Text that is not steps, or has problems, changes nothing and fails with the first problem, since a paste of
// something else (a URL, a word) is a slip, not an edit. done gets the ids the new top-level steps were given.
func (d *FlowDoc) PasteText(text, after string, done func(ids []string), fail func(error)) {
	t := d.target
	type result struct {
		flow  model.Flow
		diags diag.List
		err   error
	}
	Run(d.w, func() result {
		f, diags, err := d.b.ParseFlowSource(t, text)
		return result{f, diags, err}
	}, func(r result) {
		switch {
		case r.err != nil:
			fail(r.err)
		case r.diags.HasErrors():
			lines := problemLines(r.diags)
			fail(errors.New("the clipboard does not hold steps: " + lines[0]))
		case len(r.flow.Nodes) == 0:
			fail(errors.New("the clipboard holds no steps"))
		default:
			ids, err := d.PasteSteps(r.flow.Nodes, after)
			if err != nil {
				fail(err)
				return
			}
			done(ids)
		}
	})
}
