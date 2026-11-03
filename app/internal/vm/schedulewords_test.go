// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import "testing"

func TestDescribeCron_Given_common_schedules_When_described_Then_reads_as_a_sentence(t *testing.T) {
	cases := []struct {
		cron string
		want string
	}{
		{"@hourly", "Every hour"},
		{"@daily", "Every day at 00:00"},
		{"@midnight", "Every day at 00:00"},
		{"@weekly", "Every Sunday at 00:00"},
		{"@monthly", "On day 1 of every month at 00:00"},
		{"* * * * *", "Every minute"},
		{"*/15 * * * *", "Every 15 minutes"},
		{"0 3 * * *", "Every day at 03:00"},
		{"30 * * * *", "Every hour at :30"},
		{"5 14 * * *", "Every day at 14:05"},
		{"0 9 * * 1-5", "Weekdays at 09:00"},
		{"0 9 * * 1", "Every Monday at 09:00"},
		{"0 9 * * 7", "Every Sunday at 09:00"},
		{"0 3 1 * *", "On day 1 of every month at 03:00"},
	}

	for _, c := range cases {
		t.Run(c.cron, func(t *testing.T) {
			got := DescribeCron(c.cron)

			if got != c.want {
				t.Fatalf("DescribeCron(%q) = %q, want %q", c.cron, got, c.want)
			}
		})
	}
}

func TestDescribeCron_Given_a_pattern_it_does_not_know_When_described_Then_says_nothing(t *testing.T) {
	// A wrong sentence is worse than none: the screen then shows the cron text on its own.
	for _, cron := range []string{"", "nonsense", "0 3,15 * * *", "0 3 * 6 *", "*/7 3 * * *", "0 3 * * 1,3", "0 25 * * *", "61 * * * *"} {
		if got := DescribeCron(cron); got != "" {
			t.Fatalf("DescribeCron(%q) = %q, want no description", cron, got)
		}
	}
}
