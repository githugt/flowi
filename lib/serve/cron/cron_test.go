// SPDX-License-Identifier: Apache-2.0

package cron_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/serve/cron"
)

func at(t *testing.T, loc *time.Location, s string) time.Time {
	t.Helper()
	v, err := time.ParseInLocation("2006-01-02 15:04", s, loc)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestNext_should_find_the_next_matching_minute(t *testing.T) {
	cases := []struct{ name, expr, after, want string }{
		{"every minute", "* * * * *", "2026-10-09 12:00", "2026-10-09 12:01"},
		{"strictly after, not at", "30 3 * * *", "2026-10-09 03:30", "2026-10-10 03:30"},
		{"step", "*/15 * * * *", "2026-10-09 12:20", "2026-10-09 12:30"},
		{"range with step", "0-30/10 9 * * *", "2026-10-09 09:11", "2026-10-09 09:20"},
		{"list", "5,35 * * * *", "2026-10-09 12:06", "2026-10-09 12:35"},
		{"start with step runs to the end of the field", "10/20 * * * *", "2026-10-09 12:11", "2026-10-09 12:30"},
		{"next hour", "0 * * * *", "2026-10-09 12:30", "2026-10-09 13:00"},
		{"month names", "0 0 1 jan,JUL *", "2026-10-09 00:00", "2027-01-01 00:00"},
		{"weekday names", "0 9 * * mon-fri", "2026-10-09 10:00", "2026-10-12 09:00"}, // 2026-10-09 is a Friday
		{"7 is sunday", "0 9 * * 7", "2026-10-09 10:00", "2026-10-11 09:00"},
		{"hourly macro", "@hourly", "2026-10-09 12:30", "2026-10-09 13:00"},
		{"daily macro", "@daily", "2026-10-09 12:30", "2026-10-10 00:00"},
		{"weekly macro", "@weekly", "2026-10-09 12:30", "2026-10-11 00:00"},
		{"monthly macro", "@monthly", "2026-10-09 12:30", "2026-11-01 00:00"},
		{"yearly macro", "@yearly", "2026-10-09 12:30", "2027-01-01 00:00"},
		{"day 31 skips short months", "0 0 31 * *", "2026-10-31 00:00", "2026-12-31 00:00"},
		{"leap day", "0 0 29 2 *", "2026-10-09 00:00", "2028-02-29 00:00"},
		{"day-of-month or day-of-week when both are given", "0 0 13 * fri", "2026-10-09 12:00", "2026-10-13 00:00"},
		{"only day-of-month when weekday is a star", "0 0 13 * *", "2026-10-09 12:00", "2026-10-13 00:00"},
		{"question mark is a star", "0 0 ? * ?", "2026-10-09 12:00", "2026-10-10 00:00"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, err := cron.Parse(c.expr)
			if err != nil {
				t.Fatal(err)
			}

			got := s.Next(at(t, time.UTC, c.after))

			if want := at(t, time.UTC, c.want); !got.Equal(want) {
				t.Fatalf("Next(%s) = %s, want %s", c.after, got, want)
			}
		})
	}
}

func TestNext_should_use_the_location_of_after(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("no tz database")
	}
	s, _ := cron.Parse("0 3 * * *")

	got := s.Next(at(t, berlin, "2026-10-09 04:00"))

	if want := at(t, berlin, "2026-10-10 03:00"); !got.Equal(want) {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestNext_Given_a_time_in_the_spring_forward_gap_Then_runs_right_after_the_gap(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("no tz database")
	}
	s, _ := cron.Parse("30 2 * * *") // 2026-03-08 02:30 does not exist in New York

	got := s.Next(at(t, ny, "2026-03-08 00:00"))

	if got.Day() != 8 || got.Hour() != 3 {
		t.Fatalf("got %s, want it to run on the 8th, after the gap", got)
	}
	if again := s.Next(got); again.Day() != 9 || again.Hour() != 2 || again.Minute() != 30 {
		t.Fatalf("next after the gap run = %s", again)
	}
}

func TestNext_Given_a_repeated_hour_Then_runs_once(t *testing.T) {
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("no tz database")
	}
	s, _ := cron.Parse("30 1 * * *") // 2026-11-01 01:30 happens twice
	first := s.Next(at(t, ny, "2026-11-01 00:00"))

	second := s.Next(first)

	if second.Day() != 2 {
		t.Fatalf("first %s, second %s: the repeated 01:30 ran twice", first, second)
	}
}

func TestNext_Given_an_impossible_date_Then_zero(t *testing.T) {
	s, err := cron.Parse("0 0 30 2 *")
	if err != nil {
		t.Fatal(err)
	}

	got := s.Next(at(t, time.UTC, "2026-01-01 00:00"))

	if !got.IsZero() {
		t.Fatalf("got %s for Feb 30", got)
	}
}

func TestParse_should_explain_what_is_wrong(t *testing.T) {
	cases := []struct{ expr, field, detail string }{
		{"", "", "expected 5 fields"},
		{"* * * *", "", "expected 5 fields"},
		{"* * * * * *", "", "expected 5 fields"},
		{"60 * * * *", "minute", "60 is outside 0-59"},
		{"* 24 * * *", "hour", "24 is outside 0-23"},
		{"* * 0 * *", "day of month", "0 is outside 1-31"},
		{"* * * 13 *", "month", "13 is outside 1-12"},
		{"* * * * 8", "day of week", "8 is outside 0-7"},
		{"*/0 * * * *", "minute", "step must be at least 1"},
		{"5-1 * * * *", "minute", "range 5-1 runs backwards"},
		{"a * * * *", "minute", `"a" is not a number`},
		{"* * * foo *", "month", `"foo" is not a month`},
		{"* * * * foo", "day of week", `"foo" is not a weekday`},
		{"1,,2 * * * *", "minute", "empty list item"},
		{"@sometimes", "", "unknown macro @sometimes"},
		{"@reboot", "", "unknown macro @reboot"},
	}
	for _, c := range cases {
		t.Run(c.expr, func(t *testing.T) {
			_, err := cron.Parse(c.expr)

			var pe *cron.ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("got %v, want *ParseError", err)
			}
			if pe.Field != c.field || !strings.HasPrefix(pe.Detail, c.detail) {
				t.Fatalf("field %q detail %q, want %q / %q", pe.Field, pe.Detail, c.field, c.detail)
			}
		})
	}
}

func FuzzParse(f *testing.F) {
	for _, s := range []string{"* * * * *", "*/5 1-3 1,15 jan-mar mon", "@daily", "99999999999999999999 * * * *"} {
		f.Add(s)
	}
	f.Fuzz(func(_ *testing.T, expr string) {
		s, err := cron.Parse(expr)
		if err != nil {
			return
		}
		_ = s.Next(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) // must terminate
	})
}
