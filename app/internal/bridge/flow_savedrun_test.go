// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"testing"
	"time"

	flowi "github.com/githugt/flowi/lib"
)

func TestSavedRunViewOf_Given_steps_on_the_runs_clock_Then_the_bars_are_ordered_and_the_result_lines_read_like_a_live_run(t *testing.T) {
	// Before: the Steps list and the Timeline could only show the live run, so an earlier run's steps were out of reach.
	run := flowi.RunSteps{ID: "01A", Status: "failed", Steps: []flowi.RunStep{
		{ID: "pay", Status: "failed", Instances: 2, Code: 500, Start: time.Second, End: 1700 * time.Millisecond},
		{ID: "login", Status: "passed", Instances: 1, Code: 200, Start: 0, End: 200 * time.Millisecond},
		{ID: "note", Status: "skipped", Instances: 1},
	}}

	v := savedRunViewOf(run)

	if len(v.Timeline) != 2 || v.Timeline[0].Step != "login" || v.Timeline[1].Step != "pay" || v.Timeline[1].StartMs != 1000 || v.Timeline[1].EndMs != 1700 {
		t.Fatalf("timeline %+v", v.Timeline)
	}
	if v.Results["login"] != "200 · 200ms" || v.Results["pay"] != "failed · 500 · 2 runs · 700ms" || v.Results["note"] != "skipped" {
		t.Fatalf("results %+v", v.Results)
	}
	if v.Statuses["pay"] != "failed" || v.Statuses["note"] != "skipped" {
		t.Fatalf("statuses %+v", v.Statuses)
	}
}
