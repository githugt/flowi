// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/report"
)

// TimelineView is a finished run laid out in time, for the strip under the canvas: where each step sat and what
// it wrote, so dragging a playhead can repaint the flow as it was then.
type TimelineView struct {
	TotalMS int64     `json:"totalMS"`
	Bars    []BarView `json:"bars"`
}

// BarView is one step on the timeline, in milliseconds from the start of the run.
type BarView struct {
	ID      string      `json:"id"`
	Type    string      `json:"type"`
	Status  string      `json:"status"`
	Lane    int         `json:"lane"`
	StartMS int64       `json:"startMS"`
	EndMS   int64       `json:"endMS"`
	Wrote   []WriteView `json:"wrote"`
}

// WriteView is a variable a step set and what it set it to, as text.
type WriteView struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// timelineOf lays out a report; a run with no timing gives no bars.
func timelineOf(rep report.Run) TimelineView {
	bars := report.Timeline(rep)
	out := TimelineView{Bars: make([]BarView, 0, len(bars))}
	writes := writesByPath(rep.Nodes, map[string][]report.VarWrite{})
	for _, b := range bars {
		wrote := []WriteView{}
		for _, w := range writes[b.Path] {
			wrote = append(wrote, WriteView{Name: w.Name, Value: shortText(w.Value, maxHopText)})
		}
		out.Bars = append(out.Bars, BarView{ID: b.ID, Type: b.Type, Status: b.Status, Lane: b.Lane, StartMS: b.Start.Milliseconds(), EndMS: b.End.Milliseconds(), Wrote: wrote})
		out.TotalMS = max(out.TotalMS, b.End.Milliseconds())
	}
	out.TotalMS = max(out.TotalMS, rep.Duration.Milliseconds())
	return out
}

// writesByPath lists what every step of a report wrote, by its path.
func writesByPath(list []report.Node, out map[string][]report.VarWrite) map[string][]report.VarWrite {
	for _, n := range list {
		if len(n.Wrote) > 0 {
			out[n.Path] = n.Wrote
		}
		for _, l := range n.Lanes {
			writesByPath(l.Nodes, out)
		}
		for _, h := range n.Hooks {
			writesByPath(h.Nodes, out)
		}
	}
	return out
}

// GhostView is the bar a step had in an earlier run, for drawing behind the current bar.
type GhostView struct {
	ID      string `json:"id"`
	Lane    int    `json:"lane"`
	Status  string `json:"status"`
	StartMS int64  `json:"startMS"`
	EndMS   int64  `json:"endMS"`
	// DeltaMS is how much longer the step took in the run shown than in the earlier one (negative: quicker).
	DeltaMS int64 `json:"deltaMS"`
}

// GhostBars lays the saved run runID behind the run the flow shows, step by step, so a slower step stands out.
func (s *FlowService) GhostBars(node, runID string) ([]GhostView, error) {
	var t flowi.FlowTarget
	var shown report.Run
	if err := s.edit(node, func(o *openFlow) error {
		t, shown = o.doc.Target(), o.run.Result().Report
		return nil
	}); err != nil {
		return nil, err
	}
	earlier, err := s.app.backend.FlowRunDetail(t, runID)
	if err != nil {
		return nil, err
	}
	out := []GhostView{}
	for _, g := range report.Ghosts(shown, earlier.Report) {
		out = append(out, GhostView{ID: g.ID, Lane: g.Lane, Status: g.Status, StartMS: g.Start.Milliseconds(), EndMS: g.End.Milliseconds(), DeltaMS: g.Delta.Milliseconds()})
	}
	return out, nil
}
