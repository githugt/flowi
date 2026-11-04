// SPDX-License-Identifier: Apache-2.0

package report

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"time"
)

// Bar is one step of a run laid on a timeline: when it started and ended, as offsets from the start of the run,
// and the lane it sits in. Steps that never overlap share lane 0; a step that runs alongside another takes the
// next free lane.
type Bar struct {
	Path   string        `json:"path"`
	ID     string        `json:"id"`
	Type   string        `json:"type"`
	Status string        `json:"status"`
	Lane   int           `json:"lane"`
	Start  time.Duration `json:"start"`
	End    time.Duration `json:"end"`
}

// Timeline lays the steps of a run out in time. A container is drawn by the steps inside it. A step with no start
// time (a report from before they were kept, or a step that never started) has no bar.
func Timeline(r Run) []Bar {
	var bars []Bar
	for _, n := range leaves(r.Nodes, nil) {
		if n.Started.IsZero() || r.Started.IsZero() {
			continue
		}
		start := n.Started.Sub(r.Started)
		bars = append(bars, Bar{Path: n.Path, ID: n.ID, Type: n.Type, Status: n.Status, Start: start, End: start + n.Duration})
	}
	slices.SortStableFunc(bars, func(a, b Bar) int { return int(a.Start - b.Start) })
	assignLanes(bars)
	return bars
}

// leaves lists the steps that have no steps inside them, in the order they were reported.
func leaves(list []Node, out []Node) []Node {
	for _, n := range list {
		if len(n.Lanes) == 0 {
			out = append(out, n)
			continue
		}
		for _, l := range n.Lanes {
			out = leaves(l.Nodes, out)
		}
	}
	return out
}

// assignLanes gives each bar the lowest lane whose last bar ended before it starts.
func assignLanes(bars []Bar) {
	var laneEnd []time.Duration
	for i := range bars {
		lane := slices.IndexFunc(laneEnd, func(end time.Duration) bool { return end <= bars[i].Start })
		if lane < 0 {
			lane = len(laneEnd)
			laneEnd = append(laneEnd, 0)
		}
		laneEnd[lane] = bars[i].End
		bars[i].Lane = lane
	}
}

// WriteTimeline draws the timeline as text: one row per step with a bar of width columns for the whole run,
// the lane number when the run has more than one, and the start and length of the step.
func WriteTimeline(w io.Writer, r Run, width int) {
	bars := Timeline(r)
	if len(bars) == 0 {
		fmt.Fprintln(w, "no timing in this report")
		return
	}
	total := max(r.Duration, slices.MaxFunc(bars, func(a, b Bar) int { return int(a.End - b.End) }).End, 1)
	lanes := 1 + slices.MaxFunc(bars, func(a, b Bar) int { return a.Lane - b.Lane }).Lane
	for _, b := range bars {
		lane := ""
		if lanes > 1 {
			lane = fmt.Sprintf(" L%d", b.Lane)
		}
		fmt.Fprintf(w, "%-16s%s %s  %s +%s\n", b.ID, lane, barOf(b, total, width), mark(b.Status), roundMS(b.End-b.Start))
	}
	fmt.Fprintf(w, "total %s\n", roundMS(total))
}

// barOf draws a step as a run of █ at its place in width columns; a step too short to show still gets one cell.
func barOf(b Bar, total time.Duration, width int) string {
	from := int(int64(b.Start) * int64(width) / int64(total))
	to := max(int(int64(b.End)*int64(width)/int64(total)), from+1)
	to = min(to, width)
	from = min(from, to-1)
	return "|" + strings.Repeat("·", from) + strings.Repeat("█", to-from) + strings.Repeat("·", width-to) + "|"
}

func mark(status string) string {
	switch status {
	case "passed":
		return "✓"
	case "failed", "aborted":
		return "✗"
	}
	return "○"
}

func roundMS(d time.Duration) string {
	if d < time.Millisecond {
		return d.String()
	}
	return d.Round(time.Millisecond).String()
}

// Ghost is the bar a step had in an earlier run, for drawing behind the current run's bar of the same step.
type Ghost struct {
	Path   string        `json:"path"`
	ID     string        `json:"id"`
	Lane   int           `json:"lane"` // the lane of the step in the current run, so the two bars line up
	Start  time.Duration `json:"start"`
	End    time.Duration `json:"end"`
	Status string        `json:"status"` // how the step ended in the earlier run
	// Delta is how much longer the step took now than then; negative when it was quicker.
	Delta time.Duration `json:"delta"`
}

// Ghosts pairs the steps of current with the same steps (by path) of an earlier run. A step only one of them has is
// left out: there is nothing to line it up with. An earlier run without timing has none.
func Ghosts(current, earlier Run) []Ghost {
	before := map[string]Bar{}
	for _, b := range Timeline(earlier) {
		before[b.Path] = b
	}
	var out []Ghost
	for _, now := range Timeline(current) {
		was, ok := before[now.Path]
		if !ok {
			continue
		}
		out = append(out, Ghost{Path: now.Path, ID: now.ID, Lane: now.Lane, Start: was.Start, End: was.End, Status: was.Status,
			Delta: (now.End - now.Start) - (was.End - was.Start)})
	}
	return out
}

// Slowdown is how much longer (or shorter) a step must take than before to be worth naming: noise below 50ms, or
// within a factor of two, is not news.
const (
	slowedBy  = 50 * time.Millisecond
	slowedFor = 2
)

// SlowedSteps keeps the ghosts whose step took at least 50ms and twice as long, or half the time, than before.
func SlowedSteps(ghosts []Ghost) []Ghost {
	var out []Ghost
	for _, g := range ghosts {
		was := g.End - g.Start
		now := was + g.Delta
		if abs(g.Delta) >= slowedBy && (now >= was*slowedFor || now*slowedFor <= was) {
			out = append(out, g)
		}
	}
	return out
}

func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// WriteSlowed says which steps took much longer or shorter than in the earlier run, with both times.
func WriteSlowed(w io.Writer, slowed []Ghost) {
	if len(slowed) == 0 {
		fmt.Fprintln(w, "no step took much longer or shorter than before")
		return
	}
	for _, g := range slowed {
		was := g.End - g.Start
		fmt.Fprintf(w, "  %-14s %v → %v\n", g.Path, was.Round(time.Millisecond), (was + g.Delta).Round(time.Millisecond))
	}
}
