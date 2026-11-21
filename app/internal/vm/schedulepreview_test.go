// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"strings"
	"testing"
	"time"
)

func TestPreviewCron_should_list_the_next_firings_in_the_zone_of_the_schedule(t *testing.T) {
	// Before: a mistake in the cron line showed only after Save.
	now := time.Date(2026, 11, 17, 8, 0, 0, 0, time.UTC) // a Tuesday

	got := PreviewCron("0 9 * * 1-5", "UTC", now)

	if got.Error != "" || len(got.Next) != 3 || got.Next[0] != "Tue 09:00" || got.Next[1] != "Wed 09:00" || got.Next[2] != "Thu 09:00" {
		t.Fatalf("got %+v", got)
	}
	if got.Words == "" {
		t.Fatalf("no words for a weekday pattern: %+v", got)
	}
}

func TestPreviewCron_should_say_why_a_line_or_zone_cannot_be_used(t *testing.T) {
	now := time.Date(2026, 11, 17, 8, 0, 0, 0, time.UTC)

	bad := PreviewCron("61 * * * *", "", now)
	zone := PreviewCron("@daily", "Mars/Olympus", now)

	if bad.Error == "" || len(bad.Next) != 0 || zone.Error == "" || strings.Contains(bad.Error, "preview") {
		t.Fatalf("bad %+v zone %+v", bad, zone)
	}
}

func TestPreviewCron_Given_an_empty_line_Then_nothing_to_say(t *testing.T) {
	if got := PreviewCron("  ", "", time.Now()); got.Error != "" || got.Words != "" || len(got.Next) != 0 {
		t.Fatalf("got %+v", got)
	}
}
