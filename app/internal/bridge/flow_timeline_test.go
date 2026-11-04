// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"testing"
	"time"

	"github.com/githugt/flowi/lib/report"
)

func TestTimelineOf_should_give_bars_in_milliseconds_with_what_each_step_wrote(t *testing.T) {
	start := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	ms := time.Millisecond
	rep := report.Run{Started: start, Duration: 200 * ms, Nodes: []report.Node{
		{ID: "login", Path: "login", Type: "request", Status: "passed", Started: start, Duration: 80 * ms},
		{ID: "grab", Path: "grab", Type: "set", Status: "passed", Started: start.Add(80 * ms), Duration: 2 * ms, Wrote: []report.VarWrite{{Name: "token", Value: "t-1"}}},
	}}

	got := timelineOf(rep)

	if got.TotalMS != 200 || len(got.Bars) != 2 {
		t.Fatalf("got %+v", got)
	}
	grab := got.Bars[1]
	if grab.ID != "grab" || grab.StartMS != 80 || grab.EndMS != 82 || len(grab.Wrote) != 1 || grab.Wrote[0].Name != "token" || grab.Wrote[0].Value != "t-1" {
		t.Fatalf("grab %+v", grab)
	}
}

func TestTimelineOf_Given_a_run_without_timing_Then_no_bars_and_empty_lists_not_null(t *testing.T) {
	got := timelineOf(report.Run{})

	if got.Bars == nil || len(got.Bars) != 0 {
		t.Fatalf("bars %#v: the front end needs a list", got.Bars)
	}
}
