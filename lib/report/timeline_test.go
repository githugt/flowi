// SPDX-License-Identifier: Apache-2.0

package report

import (
	"strings"
	"testing"
	"time"
)

func timedRun() Run {
	start := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	ms := time.Millisecond
	return Run{Started: start, Duration: 300 * ms, Nodes: []Node{
		{ID: "login", Type: "request", Status: "passed", Started: start, Duration: 100 * ms},
		{ID: "both", Type: "parallel", Status: "passed", Started: start.Add(100 * ms), Duration: 150 * ms, Lanes: []Lane{
			{Label: "a", Nodes: []Node{{ID: "a1", Type: "request", Status: "passed", Started: start.Add(100 * ms), Duration: 150 * ms}}},
			{Label: "b", Nodes: []Node{{ID: "b1", Type: "request", Status: "failed", Started: start.Add(110 * ms), Duration: 40 * ms}, {ID: "b2", Type: "request", Status: "passed", Started: start.Add(150 * ms), Duration: 30 * ms}}},
		}},
		{ID: "done", Type: "note", Status: "passed", Started: start.Add(250 * ms), Duration: 5 * ms},
	}}
}

func TestTimeline_should_place_steps_that_overlap_in_different_lanes(t *testing.T) {
	bars := Timeline(timedRun())

	got := map[string]Bar{}
	for _, b := range bars {
		got[b.ID] = b
	}
	if _, ok := got["both"]; ok {
		t.Error("a container is drawn by the steps inside it, not twice")
	}
	if got["login"].Lane != 0 || got["a1"].Lane != 0 || got["done"].Lane != 0 {
		t.Errorf("sequential steps share a lane: %+v", bars)
	}
	if got["b1"].Lane != 1 || got["b2"].Lane != 1 {
		t.Errorf("the step that runs alongside a1 takes the next lane: %+v", bars)
	}
	if got["b1"].Start != 110*time.Millisecond || got["b1"].End != 150*time.Millisecond {
		t.Errorf("times are offsets from the start of the run: %+v", got["b1"])
	}
	if got["login"].Status != "passed" || got["b1"].Status != "failed" {
		t.Errorf("status: %+v", bars)
	}
}

func TestTimeline_Given_a_report_without_start_times_Then_no_bars(t *testing.T) {
	r := Run{Nodes: []Node{{ID: "a", Duration: time.Second}}}

	if bars := Timeline(r); len(bars) != 0 {
		t.Fatalf("%+v", bars)
	}
}

func TestWriteTimeline_should_draw_one_row_per_step_with_its_lane_and_times(t *testing.T) {
	var out strings.Builder

	WriteTimeline(&out, timedRun(), 30)

	text := out.String()
	for _, want := range []string{"login", "a1", "b1", "100ms", "█"} {
		if !strings.Contains(text, want) {
			t.Errorf("lacks %q:\n%s", want, text)
		}
	}
}

// withPaths gives every step the path a real report has: here, its id.
func withPaths(r Run) Run {
	var walk func(list []Node)
	walk = func(list []Node) {
		for i := range list {
			list[i].Path = list[i].ID
			for _, l := range list[i].Lanes {
				walk(l.Nodes)
			}
		}
	}
	walk(r.Nodes)
	return r
}

func slowerRun() Run {
	r := withPaths(timedRun())
	r.Nodes[0].Duration = 400 * time.Millisecond // login: 100ms → 400ms
	r.Nodes[2].Started = r.Started.Add(600 * time.Millisecond)
	return r
}

func TestGhosts_should_give_the_earlier_runs_bar_of_each_step_in_the_lane_of_the_current_one(t *testing.T) {
	now, before := slowerRun(), withPaths(timedRun())

	got := Ghosts(now, before)

	byID := map[string]Ghost{}
	for _, g := range got {
		byID[g.ID] = g
	}
	login := byID["login"]
	if login.Start != 0 || login.End != 100*time.Millisecond || login.Delta != 300*time.Millisecond {
		t.Fatalf("login ghost %+v: want the earlier 0–100ms bar and a delta of +300ms", login)
	}
	for _, bar := range Timeline(now) {
		if byID[bar.ID].Lane != bar.Lane {
			t.Fatalf("%s ghost is in lane %d, the current bar in lane %d", bar.ID, byID[bar.ID].Lane, bar.Lane)
		}
	}
}

func TestGhosts_should_leave_out_steps_only_one_of_the_runs_has(t *testing.T) {
	now, before := withPaths(timedRun()), withPaths(timedRun())
	before.Nodes = append(before.Nodes, Node{ID: "extra", Path: "extra", Type: "request", Status: "passed", Started: before.Started, Duration: time.Millisecond})
	now.Nodes = append(now.Nodes, Node{ID: "fresh", Path: "fresh", Type: "request", Status: "passed", Started: now.Started, Duration: time.Millisecond})

	got := Ghosts(now, before)

	for _, g := range got {
		if g.ID == "extra" || g.ID == "fresh" {
			t.Fatalf("ghost %+v has no counterpart", g)
		}
	}
}

func TestGhosts_Given_an_earlier_run_without_timing_Then_none(t *testing.T) {
	before := timedRun()
	before.Started = time.Time{}

	if got := Ghosts(withPaths(timedRun()), before); len(got) != 0 {
		t.Fatalf("ghosts %+v", got)
	}
}

func TestSlowedSteps_should_name_steps_that_took_much_longer_or_shorter(t *testing.T) {
	got := SlowedSteps(Ghosts(slowerRun(), withPaths(timedRun())))

	if len(got) != 1 || got[0].ID != "login" {
		t.Fatalf("slowed %+v: want only login (100ms → 400ms)", got)
	}
}

func TestWriteSlowed_should_say_each_step_with_both_times_and_when_none_moved(t *testing.T) {
	var moved, still strings.Builder

	WriteSlowed(&moved, SlowedSteps(Ghosts(slowerRun(), withPaths(timedRun()))))
	WriteSlowed(&still, nil)

	if !strings.Contains(moved.String(), "login") || !strings.Contains(moved.String(), "100ms → 400ms") {
		t.Fatalf("moved:\n%s", moved.String())
	}
	if !strings.Contains(still.String(), "no step") {
		t.Fatalf("still:\n%s", still.String())
	}
}
