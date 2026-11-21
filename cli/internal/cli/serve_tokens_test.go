// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"testing"
	"time"
)

func TestParseLifetime_should_read_durations_days_and_weeks_and_refuse_the_rest(t *testing.T) {
	// Before: there was no lifetime to give a token.
	cases := []struct {
		in   string
		want time.Duration
		bad  bool
	}{
		{in: "", want: 0},
		{in: "90m", want: 90 * time.Minute},
		{in: "12h", want: 12 * time.Hour},
		{in: "30d", want: 30 * 24 * time.Hour},
		{in: "2w", want: 14 * 24 * time.Hour},
		{in: " 1d ", want: 24 * time.Hour},
		{in: "0d", bad: true},
		{in: "-3d", bad: true},
		{in: "d", bad: true},
		{in: "5s", bad: true},
		{in: "soon", bad: true},
		{in: "99999d", bad: true},
	}
	for _, c := range cases {
		// Act
		got, err := parseLifetime(c.in)

		// Assert
		if (err != nil) != c.bad || (!c.bad && got != c.want) {
			t.Errorf("%q: got %v, %v (want %v, bad %v)", c.in, got, err, c.want, c.bad)
		}
	}
}
