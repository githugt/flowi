// SPDX-License-Identifier: Apache-2.0

package report

import (
	"fmt"
	"io"
	"slices"
)

// Change is one way a run went differently from another.
type Change struct {
	Node string `json:"node"`
	What string `json:"what"` // "status", "wrote <variable>" or "step"
	From string `json:"from"`
	To   string `json:"to"`
}

// Comparison says how run b went differently from run a, in the order of the steps of b, then the steps only a ran.
type Comparison struct {
	// FirstDivergence is the first step with a change; "" when the runs match.
	FirstDivergence string   `json:"first_divergence,omitempty"`
	Changes         []Change `json:"changes"`
}

// Compare lays two runs of a flow side by side: step by step, did it end the same way, did it write the same
// values, did it run at all. Times are not compared: two runs of one flow never take exactly as long. A step that
// ran several times (in a batch) is compared by its last run.
func Compare(a, b Run) Comparison {
	left, right := lastByID(steps(a.Nodes, nil)), lastByID(steps(b.Nodes, nil))
	out := Comparison{Changes: []Change{}}
	for _, id := range orderOf(b.Nodes, a.Nodes) {
		l, inA := left[id]
		r, inB := right[id]
		switch {
		case !inA:
			out.Changes = append(out.Changes, Change{Node: id, What: "step", From: "missing", To: "ran"})
		case !inB:
			out.Changes = append(out.Changes, Change{Node: id, What: "step", From: "ran", To: "missing"})
		default:
			out.Changes = append(out.Changes, differences(l, r)...)
		}
	}
	if len(out.Changes) > 0 {
		out.FirstDivergence = out.Changes[0].Node
	}
	return out
}

// differences are the changes of step r against step l.
func differences(l, r Node) []Change {
	var out []Change
	if l.Status != r.Status {
		out = append(out, Change{Node: r.ID, What: "status", From: l.Status, To: r.Status})
	}
	for _, w := range r.Wrote {
		was, _ := writtenValue(l, w.Name)
		if fmt.Sprint(was) != fmt.Sprint(w.Value) {
			out = append(out, Change{Node: r.ID, What: "wrote " + w.Name, From: fmt.Sprint(was), To: fmt.Sprint(w.Value)})
		}
	}
	return out
}

func writtenValue(n Node, name string) (any, bool) {
	for i := len(n.Wrote) - 1; i >= 0; i-- {
		if n.Wrote[i].Name == name {
			return n.Wrote[i].Value, true
		}
	}
	return nil, false
}

// steps lists every step of a report, containers' steps included, in the order they started.
func steps(list []Node, out []Node) []Node {
	for _, n := range list {
		out = append(out, n)
		for _, l := range n.Lanes {
			out = steps(l.Nodes, out)
		}
	}
	return out
}

func lastByID(list []Node) map[string]Node {
	out := map[string]Node{}
	for _, n := range list {
		out[n.ID] = n
	}
	return out
}

// orderOf is the step ids of the first run's report in its order, then those only the second run has.
func orderOf(first, second []Node) []string {
	var ids []string
	for _, n := range append(steps(first, nil), steps(second, nil)...) {
		if !slices.Contains(ids, n.ID) {
			ids = append(ids, n.ID)
		}
	}
	return ids
}

// WriteComparison prints a comparison: that the runs match, or the first step that differs and each change.
func WriteComparison(w io.Writer, c Comparison) {
	if len(c.Changes) == 0 {
		fmt.Fprintln(w, "the runs went the same way")
		return
	}
	fmt.Fprintf(w, "first differs at %s\n", c.FirstDivergence)
	for _, ch := range c.Changes {
		fmt.Fprintf(w, "  %-14s %-18s %s → %s\n", ch.Node, ch.What, ch.From, ch.To)
	}
}
