// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"testing"
	"time"

	"github.com/githugt/flowi/app/internal/vm"
)

// Before: a run showed one duration per card, so nothing said which steps ran side by side or where the time went.
func TestTimelineOf_should_place_steps_on_the_run_clock_in_start_order(t *testing.T) {
	began := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	details := map[string]vm.NodeDetail{
		"b": {Status: "passed", Began: began.Add(50 * time.Millisecond), Ended: began.Add(200 * time.Millisecond)},
		"a": {Status: "passed", Began: began, Ended: began.Add(120 * time.Millisecond)},
		"c": {Status: "running", Began: began.Add(300 * time.Millisecond)},
	}
	detail := func(id string) (vm.NodeDetail, bool) { d, ok := details[id]; return d, ok }

	got := timelineOf([]string{"b", "a", "c"}, detail, began, began.Add(500*time.Millisecond))

	want := []TimelineBar{
		{Step: "a", Status: "passed", StartMs: 0, EndMs: 120},
		{Step: "b", Status: "passed", StartMs: 50, EndMs: 200},
		{Step: "c", Status: "running", StartMs: 300, EndMs: 500},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("bar %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestTimelineOf_should_leave_out_steps_that_did_not_run_in_this_run(t *testing.T) {
	began := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	details := map[string]vm.NodeDetail{
		"old":     {Status: "passed", Began: began.Add(-time.Minute), Ended: began.Add(-time.Minute + time.Second)}, // kept from an earlier run
		"skipped": {Status: "skipped"},
	}
	detail := func(id string) (vm.NodeDetail, bool) { d, ok := details[id]; return d, ok }

	got := timelineOf([]string{"old", "skipped", "missing"}, detail, began, began)
	none := timelineOf([]string{"old"}, detail, time.Time{}, began)

	if len(got) != 0 || len(none) != 0 {
		t.Fatalf("got %+v / %+v, want no bars", got, none)
	}
}
