// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"testing"
	"time"

	"github.com/githugt/flowi/lib/model"
)

func TestScheduleOutlook_Given_a_daily_schedule_When_looked_at_Then_next_run_and_one_firing_in_a_day(t *testing.T) {
	now := time.Date(2026, 11, 2, 22, 48, 0, 0, time.UTC)
	s := model.Schedule{Name: "nightly", Flow: "login", Cron: "0 3 * * *", TZ: "UTC"}

	next, fires := ScheduleOutlook(s, now)

	if want := time.Date(2026, 11, 3, 3, 0, 0, 0, time.UTC); !next.Equal(want) {
		t.Fatalf("next = %v, want %v", next, want)
	}
	if len(fires) != 1 || fires[0] != 4*60+12 {
		t.Fatalf("fires = %v, want one firing 252 minutes ahead", fires)
	}
}

func TestScheduleOutlook_Given_every_15_minutes_When_looked_at_Then_firings_cover_the_next_24_hours(t *testing.T) {
	now := time.Date(2026, 11, 2, 10, 1, 0, 0, time.UTC)
	s := model.Schedule{Name: "smoke", Flow: "login", Cron: "*/15 * * * *", TZ: "UTC"}

	_, fires := ScheduleOutlook(s, now)

	if len(fires) != 96 || fires[0] != 14 || fires[95] != 14+95*15 {
		t.Fatalf("got %d firings, first %d, last %d", len(fires), fires[0], fires[len(fires)-1])
	}
}

func TestScheduleOutlook_Given_a_disabled_or_invalid_schedule_When_looked_at_Then_nothing_is_due(t *testing.T) {
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	for _, s := range []model.Schedule{
		{Name: "off", Flow: "login", Cron: "@hourly", Disabled: true},
		{Name: "bad", Flow: "login", Cron: "not cron"},
	} {
		next, fires := ScheduleOutlook(s, now)

		if !next.IsZero() || len(fires) != 0 {
			t.Fatalf("%s: next %v, fires %v", s.Name, next, fires)
		}
	}
}
