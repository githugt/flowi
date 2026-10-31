// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"fmt"
	"time"

	flowi "github.com/githugt/flowi/lib"
)

// RunHistory is the list of a flow's saved runs, newest first, and the
// comparison of any of them with the newest.
type RunHistory struct {
	b        Backend
	w        *Worker
	target   flowi.FlowTarget
	runs     []flowi.RunSummary
	onChange []func()
}

// NewRunHistory returns the history of the flow doc holds. Call Load to fill it.
func NewRunHistory(b Backend, w *Worker, doc *FlowDoc) *RunHistory {
	return &RunHistory{b: b, w: w, target: doc.Target()}
}

// OnChange registers f to run when the list is reloaded.
func (h *RunHistory) OnChange(f func()) { h.onChange = append(h.onChange, f) }

// Load reads the saved runs again, off the UI goroutine.
func (h *RunHistory) Load() {
	Run(h.w, func() []flowi.RunSummary { return h.b.FlowRuns(h.target, 0) }, func(runs []flowi.RunSummary) {
		h.runs = runs
		for _, f := range h.onChange {
			f()
		}
	})
}

// Runs are the saved runs, newest first.
func (h *RunHistory) Runs() []flowi.RunSummary { return h.runs }

// CompareWithLatest compares run i with the newest run; nil for the newest
// itself or an index out of range.
func (h *RunHistory) CompareWithLatest(i int) []DiffRow {
	if i <= 0 || i >= len(h.runs) {
		return nil
	}
	return CompareRuns(h.runs[i], h.runs[0])
}

// Between compares run from with run to (indexes into Runs): the statuses and
// times of their top-level nodes. nil for an index out of range.
func (h *RunHistory) Between(from, to int) []DiffRow {
	if from < 0 || to < 0 || from >= len(h.runs) || to >= len(h.runs) {
		return nil
	}
	return CompareRuns(h.runs[from], h.runs[to])
}

// CompareInDepth reads both runs and compares what each node output and the
// variables the flow ended with, off the UI goroutine.
func (h *RunHistory) CompareInDepth(from, to int, done func(RunComparison, error)) {
	if from < 0 || to < 0 || from >= len(h.runs) || to >= len(h.runs) {
		done(RunComparison{}, nil)
		return
	}
	a, b, t := h.runs[from].ID, h.runs[to].ID, h.target
	type result struct {
		c   RunComparison
		err error
	}
	Run(h.w, func() result {
		before, err := h.b.FlowRunDetail(t, a)
		if err != nil {
			return result{err: err}
		}
		after, err := h.b.FlowRunDetail(t, b)
		return result{CompareDetails(before, after), err}
	}, func(r result) { done(r.c, r.err) })
}

// RunLabel is one line for a run in the list: "passed · 14:03 · 3.2s · local".
func RunLabel(s flowi.RunSummary) string {
	label := s.Status
	if !s.Started.IsZero() {
		label += " · " + s.Started.Local().Format("15:04:05")
	}
	label += " · " + s.Duration.Round(10*time.Millisecond).String()
	if s.Env != "" {
		label += " · " + s.Env
	}
	return label
}

// DiffRow is one node in the comparison of two runs: what it did in each and
// how much its time moved.
type DiffRow struct {
	Node        string
	Name        string
	Before      string // status in the older run, "" when it was not in it
	After       string
	DurationGap string // "+120ms", "-40ms", "" when both ran and the times match
	Changed     bool   // the status differs
}

// CompareRuns lines up the top-level nodes of before and after: those of after
// in its order, then those only before had.
func CompareRuns(before, after flowi.RunSummary) []DiffRow {
	seen := map[string]flowi.RunNodeSummary{}
	for _, n := range before.Nodes {
		seen[n.ID] = n
	}
	var rows []DiffRow
	for _, n := range after.Nodes {
		old, had := seen[n.ID]
		delete(seen, n.ID)
		row := DiffRow{Node: n.ID, Name: n.Name, After: n.Status}
		if had {
			row.Before = old.Status
			row.DurationGap = durationGap(old.Duration, n.Duration)
		}
		row.Changed = row.Before != row.After
		rows = append(rows, row)
	}
	for _, n := range before.Nodes {
		if _, left := seen[n.ID]; left {
			rows = append(rows, DiffRow{Node: n.ID, Name: n.Name, Before: n.Status, Changed: true})
		}
	}
	return rows
}

func durationGap(before, after time.Duration) string {
	gap := after - before
	if gap == 0 {
		return ""
	}
	sign := "+"
	if gap < 0 {
		sign, gap = "-", -gap
	}
	return fmt.Sprintf("%s%s", sign, gap.Round(time.Millisecond))
}
