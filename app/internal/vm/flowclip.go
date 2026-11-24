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
//
// portable writes each request and flow a step calls by its path alone, with no id, and fills in the step's name and
// description: the ids belong to this collection, so a copy that carries them says nothing in another one, while a
// path can be looked up there. Pasting resolves each path (see PasteText).
func (d *FlowDoc) CopyText(ids []string, portable bool, done func(text string), fail func(error)) {
	nodes, err := d.CopySteps(ids)
	if err != nil {
		fail(err)
		return
	}
	if portable {
		makePortable(d.coll, nodes)
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

// makePortable rewrites the steps in list, and everything inside them, so each request or flow they call by this
// collection's own reference is named by its path only, and each step says in words what it calls.
func makePortable(c model.Collection, list []model.Node) {
	for i := range list {
		n := &list[i]
		FillStepText(c, n)
		if n.Request != nil && n.Request.Request.Collection == "" {
			call := *n.Request
			call.Request = portableRef(call.Request, requestPaths(c))
			n.Request = &call
		}
		if n.Subflow != nil && n.Subflow.Flow.Collection == "" {
			call := *n.Subflow
			call.Flow = portableRef(call.Flow, flowPaths(c))
			n.Subflow = &call
		}
		for _, child := range listsOf(n) {
			makePortable(c, *child)
		}
	}
}

// portableRef is ref without its id, its path brought up to date from paths (by id) when the target is known.
func portableRef(ref model.Ref, paths map[model.ID]string) model.Ref {
	if p, ok := paths[ref.ID]; ok {
		ref.Path = p
	}
	ref.ID = ""
	return ref
}

func requestPaths(c model.Collection) map[model.ID]string {
	out := map[model.ID]string{}
	for _, r := range c.Requests {
		out[r.ID] = r.Path
	}
	return out
}

func flowPaths(c model.Collection) map[model.ID]string {
	out := map[model.ID]string{}
	for _, f := range c.Flows {
		out[f.ID] = f.Path
	}
	return out
}

// resolveCalls points each request and flow that list (and what is inside it) calls at this collection's own: by id
// when the collection has it, else by its path. A call it finds nothing for is left as written, so the person can see
// what it was and pick another; its path (or id, when it has no path) is returned, once each, in the order met.
// Calls into a dependency are not touched.
func resolveCalls(c model.Collection, list []model.Node) (unmatched []string) {
	requests, flows := requestPaths(c), flowPaths(c)
	byPath := func(paths map[model.ID]string) map[string]model.ID {
		out := map[string]model.ID{}
		for id, p := range paths {
			out[p] = id
		}
		return out
	}
	requestIDs, flowIDs := byPath(requests), byPath(flows)
	resolve := func(ref model.Ref, paths map[model.ID]string, ids map[string]model.ID) model.Ref {
		if ref.Collection != "" || ref.Param != "" {
			return ref
		}
		if p, ok := paths[ref.ID]; ok && ref.ID != "" {
			ref.Path = p
			return ref
		}
		if id, ok := ids[ref.Path]; ok && ref.Path != "" {
			ref.ID = id
			return ref
		}
		if name := firstText(ref.Path, string(ref.ID)); name != "" && !slices.Contains(unmatched, name) {
			unmatched = append(unmatched, name)
		}
		return ref
	}
	var walk func(list []model.Node)
	walk = func(list []model.Node) {
		for i := range list {
			n := &list[i]
			if n.Request != nil {
				call := *n.Request
				call.Request = resolve(call.Request, requests, requestIDs)
				n.Request = &call
			}
			if n.Subflow != nil {
				call := *n.Subflow
				call.Flow = resolve(call.Flow, flows, flowIDs)
				n.Subflow = &call
			}
			for _, child := range listsOf(n) {
				walk(*child)
			}
		}
	}
	walk(list)
	return unmatched
}

// PasteText reads text as copied steps and puts them in after step `after` (the end when it is empty or not found).
// Text that is not steps, or has problems, changes nothing and fails with the first problem, since a paste of
// something else (a URL, a word) is a slip, not an edit. done gets the ids the new top-level steps were given, and
// the requests and flows this collection has no match for (by id, then by path): those steps are pasted all the same,
// still pointing at what they were copied with.
func (d *FlowDoc) PasteText(text, after string, done func(ids []string, unmatched []string), fail func(error)) {
	t := d.target
	type result struct {
		flow  model.Flow
		diags diag.List
		err   error
	}
	Run(d.w, func() result {
		f, diags, err := d.b.ParseFlowSnippet(t, text)
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
			unmatched := resolveCalls(d.coll, r.flow.Nodes)
			ids, err := d.PasteSteps(r.flow.Nodes, after)
			if err != nil {
				fail(err)
				return
			}
			done(ids, unmatched)
		}
	})
}
