// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"testing"
	"time"
)

func TestTookText_should_pick_the_unit_that_reads_best(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   time.Duration
		want string
	}{
		{"nothing measured", 0, ""},
		{"microseconds", 820 * time.Microsecond, "820 µs"},
		{"milliseconds", 142 * time.Millisecond, "142 ms"},
		{"just under a second", 999 * time.Millisecond, "999 ms"},
		{"seconds", 1240 * time.Millisecond, "1.24 s"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := TookText(tc.in)

			if got != tc.want {
				t.Errorf("TookText(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSizeText_should_scale_bytes_and_show_an_empty_body(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   int64
		want string
	}{
		{"unknown", -1, ""},
		{"empty", 0, "0 B"},
		{"bytes", 512, "512 B"},
		{"kilobytes", 1234, "1.2 KB"},
		{"megabytes", 3_400_000, "3.4 MB"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := SizeText(tc.in)

			if got != tc.want {
				t.Errorf("SizeText(%d) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
