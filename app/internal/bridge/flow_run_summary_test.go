// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import "testing"

func TestRunSummary_should_count_the_steps_by_how_they_ended(t *testing.T) {
	cases := []struct {
		name   string
		status string
		steps  map[string]string
		want   string
	}{
		{"all passed", "passed", map[string]string{"a": "passed", "b": "passed"}, "passed · 2 passed · seed 7"},
		{"a failure", "failed", map[string]string{"a": "passed", "b": "failed", "c": "error", "d": "skipped"}, "failed · 1 passed, 2 failed, 1 not run · seed 7"},
		{"nothing ran", "cancelled", map[string]string{}, "cancelled · seed 7"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := runSummary(c.status, 7, c.steps); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}
