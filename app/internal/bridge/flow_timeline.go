// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"slices"
	"time"

	"github.com/githugt/flowi/app/internal/vm"
)

// TimelineBar is one step of the last run placed on the run's clock: how long after the run began it started and
// ended, in milliseconds, so the canvas can draw a waterfall of the run.
type TimelineBar struct {
	Step    string  `json:"step"`
	Status  string  `json:"status"`
	StartMs float64 `json:"startMs"`
	EndMs   float64 `json:"endMs"`
}

// timelineOf places the steps ids that ran after began on the run's clock, ordered by start. A step with no start
// (it did not run, or its results are from an earlier run) is left out; a step still running ends at now.
// Before: the run showed one duration per card and nothing said which steps ran side by side or where the time went.
func timelineOf(ids []string, detail func(string) (vm.NodeDetail, bool), began, now time.Time) []TimelineBar {
	out := []TimelineBar{}
	if began.IsZero() {
		return out
	}
	for _, id := range ids {
		d, ok := detail(id)
		if !ok || d.Began.IsZero() || d.Began.Before(began) {
			continue
		}
		end := d.Ended
		if end.IsZero() { // still running
			end = now
		}
		out = append(out, TimelineBar{Step: id, Status: d.Status, StartMs: ms(d.Began.Sub(began)), EndMs: ms(max(end.Sub(began), d.Began.Sub(began)))})
	}
	slices.SortStableFunc(out, func(a, b TimelineBar) int {
		switch {
		case a.StartMs < b.StartMs:
			return -1
		case a.StartMs > b.StartMs:
			return 1
		}
		return 0
	})
	return out
}
