// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/model"
)

// wrapKinds are the containers a run of steps can be wrapped in, with the node type each is.
func wrapKinds() map[string]model.NodeType {
	return map[string]model.NodeType{"loop": "batch", "group": "parallel", "route": "condition"}
}

// Wrap puts the steps ids, which must sit next to each other in one list, inside a new container of kind (loop, group
// or route) that takes their place, and returns the container's id and the other steps that read the wrapped steps'
// results. It is one undo step.
//
// The wrapped steps keep their order. Arrows into the run from outside move to the container, and arrows from later
// steps to a wrapped step now point at the container. A group holds them in its first lane (the second is empty, for
// the person to fill); a route holds them in a first case whose condition is "true", so nothing changes until it is
// edited. A loop has no list to repeat over yet: the person says what to repeat.
//
// Before: selecting several steps offered only Delete, so putting them in a loop meant adding the container, dragging
// each step into it and mending the arrows by hand.
func (d *FlowDoc) Wrap(ids []string, kind string) (string, []string, error) {
	typ, ok := wrapKinds()[kind]
	if !ok {
		return "", nil, fmt.Errorf("a step can be wrapped in a loop, a group or a route, not %q", kind)
	}
	var made string
	var readBy []string
	err := d.command(func(f *model.Flow) error {
		list, lo, hi, err := wrapRun(f, ids)
		if err != nil {
			return err
		}
		run := slices.Clone((*list)[lo : hi+1])
		inside := map[string]bool{}
		for _, n := range run {
			inside[n.ID] = true
		}
		readBy = readersOf(*f, inside)
		taken := map[string]bool{}
		allIDs(f.Nodes, taken)
		box := newNode(typ, taken)
		pullArrows(&box, run, inside)
		redirectArrows(f.Nodes, inside, box.ID)
		switch typ {
		case "batch":
			box.Body = run
		case "parallel":
			box.Branches[0].Nodes = run
		default:
			box.Cases = []model.Case{{When: "true", Nodes: run}}
		}
		*list = slices.Replace(*list, lo, hi+1, box)
		made = box.ID
		return nil
	})
	return made, readBy, err
}

// ErrWrapApart refuses wrapping steps that are not in one unbroken run of one list.
var ErrWrapApart = errors.New("select steps that sit next to each other in the same list to wrap them")

// wrapRun finds the list the steps are in and the first and last index of their run; the steps must fill the run.
func wrapRun(f *model.Flow, ids []string) (list *[]model.Node, lo, hi int, err error) {
	ids = slices.Compact(slices.Sorted(slices.Values(ids)))
	if len(ids) == 0 {
		return nil, 0, 0, errors.New("select the steps to wrap first")
	}
	var at []int
	path := ""
	for _, id := range ids {
		p, i, ok := locate(f.Nodes, "$.nodes", id)
		if !ok {
			return nil, 0, 0, fmt.Errorf("no step %s", id)
		}
		if path != "" && p != path {
			return nil, 0, 0, ErrWrapApart
		}
		path = p
		at = append(at, i)
	}
	slices.Sort(at)
	if at[len(at)-1]-at[0] != len(at)-1 {
		return nil, 0, 0, ErrWrapApart
	}
	list, ok := listAt(f, path)
	if !ok {
		return nil, 0, 0, fmt.Errorf("there is no list of steps at %s", path)
	}
	return list, at[0], at[len(at)-1], nil
}

// readersOf are the steps outside inside whose fields read the results of a step inside, sorted.
func readersOf(f model.Flow, inside map[string]bool) []string {
	seen := map[string]bool{}
	var out []string
	for id := range inside {
		for _, r := range stepsReading(f, id, inside) {
			if !seen[r] {
				seen[r] = true
				out = append(out, r)
			}
		}
	}
	slices.Sort(out)
	return out
}

// pullArrows moves the arrows that point out of the run (a step waiting on a step before it) from the wrapped steps to
// the container, and takes the "starts a chain" mark of the first step to it. A drawing attachment to a step outside
// the run is dropped: the container is not next to that step any more.
func pullArrows(box *model.Node, run []model.Node, inside map[string]bool) {
	for i := range run {
		n := &run[i]
		var kept []string
		kept, box.Needs = splitOut(n.Needs, box.Needs, inside)
		n.Needs = kept
		kept, box.AfterFailureOf = splitOut(n.AfterFailureOf, box.AfterFailureOf, inside)
		n.AfterFailureOf = kept
		n.AfterOutcome, box.AfterOutcome = splitOutcomes(n.AfterOutcome, box.AfterOutcome, inside)
		if n.Attach != "" && !inside[n.Attach] {
			n.Attach, n.AttachAt = "", ""
		}
	}
	if len(run) > 0 && run[0].Unlinked {
		box.Unlinked, run[0].Unlinked = true, false
	}
}

// splitOut keeps the names of from that are inside and adds the others to to, once each.
func splitOut(from, to []string, inside map[string]bool) (kept, out []string) {
	out = to
	for _, name := range from {
		switch {
		case inside[name]:
			kept = append(kept, name)
		case !slices.Contains(out, name):
			out = append(out, name)
		}
	}
	return kept, out
}

// splitOutcomes is splitOut for "step:outcome" entries.
func splitOutcomes(from, to []string, inside map[string]bool) (kept, out []string) {
	out = to
	for _, entry := range from {
		step, _, _ := strings.Cut(entry, ":")
		switch {
		case inside[step]:
			kept = append(kept, entry)
		case !slices.Contains(out, entry):
			out = append(out, entry)
		}
	}
	return kept, out
}

// redirectArrows makes every step outside the run that waited on a step of the run wait on the container instead, and
// stops drawing a step with a wrapped request it was attached to.
func redirectArrows(list []model.Node, inside map[string]bool, box string) {
	for i := range list {
		n := &list[i]
		if !inside[n.ID] {
			n.Needs = redirect(n.Needs, inside, box)
			n.AfterFailureOf = redirect(n.AfterFailureOf, inside, box)
			n.AfterOutcome = redirectOutcomes(n.AfterOutcome, inside, box)
			if inside[n.Attach] {
				n.Attach, n.AttachAt = "", ""
			}
		}
		for _, child := range listsOf(n) {
			redirectArrows(*child, inside, box)
		}
	}
}

func redirect(names []string, inside map[string]bool, box string) []string {
	var out []string
	for _, name := range names {
		if inside[name] {
			name = box
		}
		if !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	return out
}

func redirectOutcomes(entries []string, inside map[string]bool, box string) []string {
	var out []string
	for _, entry := range entries {
		step, outcome, ok := strings.Cut(entry, ":")
		if ok && inside[step] {
			entry = box + ":" + outcome
		}
		if !slices.Contains(out, entry) {
			out = append(out, entry)
		}
	}
	return out
}
