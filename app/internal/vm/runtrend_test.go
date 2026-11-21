// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"testing"
	"time"

	flowi "github.com/githugt/flowi/lib"
)

func run(status string, d time.Duration, nodes ...flowi.RunNodeSummary) flowi.RunSummary {
	return flowi.RunSummary{Status: status, Duration: d, Nodes: nodes}
}

func ms(n int) time.Duration { return time.Duration(n) * time.Millisecond }

// Before: the runs were a list and nothing added them up.
func TestRunTrendOf_Given_runs_that_flip_Then_the_strip_runs_oldest_first_and_the_flow_is_flaky(t *testing.T) {
	// newest first, as the history keeps them
	runs := []flowi.RunSummary{run("passed", ms(100)), run("failed", ms(100)), run("passed", ms(100)), run("passed", ms(100))}

	got := RunTrendOf(runs)

	if got.Strip != "✓✓✗✓" || got.Flips != 2 || !got.Flaky {
		t.Fatalf("%+v", got)
	}
}

func TestRunTrendOf_Given_one_failure_in_a_row_of_passes_Then_it_is_not_called_flaky(t *testing.T) {
	runs := []flowi.RunSummary{run("failed", ms(100)), run("passed", ms(100)), run("passed", ms(100))}

	got := RunTrendOf(runs)

	if got.Flips != 1 || got.Flaky {
		t.Fatalf("a flow that started failing is broken, not flaky: %+v", got)
	}
}

func TestRunTrendOf_Given_a_slow_latest_run_Then_it_is_slower_than_usual_and_names_the_step(t *testing.T) {
	step := func(d int) flowi.RunNodeSummary { return flowi.RunNodeSummary{ID: "s4", Name: "pay", Duration: ms(d)} }
	runs := []flowi.RunSummary{run("passed", ms(2000), step(1900)), run("passed", ms(800), step(700)), run("passed", ms(900), step(800)), run("passed", ms(850), step(750))}

	got := RunTrendOf(runs)

	if !got.Slow || got.SlowStep != "pay" || got.SlowFactor < 2.4 || got.SlowFactor > 2.7 {
		t.Fatalf("%+v", got)
	}
	if note := got.SlowNote(); note != "pay took 2.5 times its usual time in the latest run." {
		t.Fatalf("note %q", note)
	}
}

func TestRunTrendOf_Given_a_steady_flow_Then_nothing_stands_out(t *testing.T) {
	runs := []flowi.RunSummary{run("passed", ms(820)), run("passed", ms(800)), run("passed", ms(810))}

	got := RunTrendOf(runs)

	if got.Slow || got.Flaky || got.SlowNote() != "" || got.Text() != "✓✓✓ · median 810 ms" {
		t.Fatalf("%+v %q", got, got.Text())
	}
}

func TestRunTrendOf_Given_no_runs_Then_it_is_empty(t *testing.T) {
	if got := RunTrendOf(nil); got.Text() != "" || got.Strip != "" {
		t.Fatalf("%+v", got)
	}
}

func TestRunTrendOf_Given_more_than_ten_runs_Then_only_the_latest_ten_count(t *testing.T) {
	var runs []flowi.RunSummary
	for i := 0; i < 15; i++ {
		runs = append(runs, run("passed", ms(100)))
	}

	if got := RunTrendOf(runs); len([]rune(got.Strip)) != TrendRuns {
		t.Fatalf("strip %q", got.Strip)
	}
}
