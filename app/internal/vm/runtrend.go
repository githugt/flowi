// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"fmt"
	"slices"
	"time"

	flowi "github.com/githugt/flowi/lib"
)

// TrendRuns is how many of the latest runs the trend looks at.
const TrendRuns = 10

// RunTrend is what the saved runs of a flow say together: how the last ones ended, whether the flow flips between
// passing and failing, how long it usually takes and whether its latest run was slower.
//
// Before: the runs were a list, and nothing added them up, so a flaky flow or one that was getting slower looked like
// any other.
type RunTrend struct {
	// Strip is one character for each of the latest runs, oldest first: "✓" passed, "✗" failed, "·" anything else.
	Strip string
	// Flips counts the changes between passing and failing in the strip.
	Flips int
	// Flaky is true when the flow has both passed and failed in these runs and flipped at least twice.
	Flaky bool
	// Median is the middle duration of the runs; zero for no runs.
	Median time.Duration
	// Slow says the latest run took at least 1.5 times the median of the runs before it, and by at least 100 ms.
	Slow bool
	// SlowStep names the step of the latest run that is furthest over its own median, and Factor says by how much.
	SlowStep   string
	SlowFactor float64
}

// RunTrendOf looks at the latest TrendRuns of runs (newest first, as RunHistory keeps them).
func RunTrendOf(runs []flowi.RunSummary) RunTrend {
	if len(runs) > TrendRuns {
		runs = runs[:TrendRuns]
	}
	var t RunTrend
	if len(runs) == 0 {
		return t
	}
	var strip []rune
	var passed, failed bool
	last := ""
	for i := len(runs) - 1; i >= 0; i-- { // oldest first
		mark, kind := '·', ""
		switch runs[i].Status {
		case "passed":
			mark, kind, passed = '✓', "pass", true
		case "failed":
			mark, kind, failed = '✗', "fail", true
		}
		strip = append(strip, mark)
		if kind != "" {
			if last != "" && kind != last {
				t.Flips++
			}
			last = kind
		}
	}
	t.Strip = string(strip)
	t.Flaky = passed && failed && t.Flips >= 2
	t.Median = medianDuration(runs)
	if len(runs) >= 3 {
		before := medianDuration(runs[1:])
		d := runs[0].Duration
		t.Slow = before > 0 && d >= before*3/2 && d-before >= 100*time.Millisecond
	}
	t.SlowStep, t.SlowFactor = slowestStep(runs)
	return t
}

func medianDuration(runs []flowi.RunSummary) time.Duration {
	if len(runs) == 0 {
		return 0
	}
	ds := make([]time.Duration, len(runs))
	for i, r := range runs {
		ds[i] = r.Duration
	}
	slices.Sort(ds)
	return ds[len(ds)/2]
}

// slowestStep finds the step of the latest run that is furthest over the median of the same step in the runs before
// it. Steps with fewer than two earlier measurements, or under 100 ms, are not judged.
func slowestStep(runs []flowi.RunSummary) (string, float64) {
	if len(runs) < 3 {
		return "", 0
	}
	earlier := map[string][]time.Duration{}
	for _, r := range runs[1:] {
		for _, n := range r.Nodes {
			if n.Duration > 0 {
				earlier[n.ID] = append(earlier[n.ID], n.Duration)
			}
		}
	}
	name, factor := "", 0.0
	for _, n := range runs[0].Nodes {
		ds := earlier[n.ID]
		if len(ds) < 2 || n.Duration < 100*time.Millisecond {
			continue
		}
		slices.Sort(ds)
		med := ds[len(ds)/2]
		if med <= 0 {
			continue
		}
		if f := float64(n.Duration) / float64(med); f > factor {
			name, factor = n.Name, f
			if name == "" {
				name = n.ID
			}
		}
	}
	return name, factor
}

// Text is the trend as one line for the Runs tab: "✓✓✗✓✓ · flaky · median 1.2 s · slower than usual".
func (t RunTrend) Text() string {
	if t.Strip == "" {
		return ""
	}
	s := t.Strip
	if t.Flaky {
		s += " · flaky"
	}
	if t.Median > 0 {
		s += " · median " + TookText(t.Median)
	}
	if t.Slow {
		s += " · slower than usual"
	}
	return s
}

// SlowNote names the step that is furthest over its median, or "" when none stands out (under 1.5 times).
func (t RunTrend) SlowNote() string {
	if t.SlowStep == "" || t.SlowFactor < 1.5 {
		return ""
	}
	return fmt.Sprintf("%s took %.1f times its usual time in the latest run.", t.SlowStep, t.SlowFactor)
}
