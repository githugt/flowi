// SPDX-License-Identifier: Apache-2.0

package version

import (
	"errors"
	"testing"
)

func mustParse(t *testing.T, s string) Label {
	t.Helper()
	l, err := Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	return l
}

func TestParse(t *testing.T) {
	valid := []string{"1", "1.1", "2.0.1", "0", "10.20"}
	invalid := []string{"", "v1", "1.", ".1", "01", "1..2", " 1", "1.-1", "99999999999999999999"}

	for _, s := range valid {
		t.Run("valid_"+s, func(t *testing.T) {
			l, err := Parse(s)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if l.String() != s {
				t.Fatalf("String() = %q, want %q", l.String(), s)
			}
		})
	}
	for _, s := range invalid {
		t.Run("invalid_"+s, func(t *testing.T) {
			_, err := Parse(s)

			if !errors.Is(err, ErrInvalidLabel) {
				t.Fatalf("got %v, want ErrInvalidLabel", err)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1", "1.1", -1},
		{"1.1", "1.10", -1},
		{"1.10", "2", -1},
		{"1", "1.0", 0},
		{"2.0.1", "2", 1},
		{"2", "1.10", 1},
	}
	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := mustParse(t, tt.a).Compare(mustParse(t, tt.b))

			if got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestHighest_should_return_false_for_empty(t *testing.T) {
	_, ok := Highest(nil)

	if ok {
		t.Fatal("ok = true for empty input")
	}
}

func TestNormalize_should_strip_the_word_in_front_of_the_number(t *testing.T) {
	cases := map[string]string{
		"2":            "2",
		"v2":           "2",
		"V.2":          "2",
		"ver2":         "2",
		"Ver. 1.2":     "1.2",
		"version 3":    "3",
		"Version: 1.0": "1.0",
		"  1.4  ":      "1.4",
	}
	for raw, want := range cases {
		t.Run(raw, func(t *testing.T) {
			got := Normalize(raw)

			if got != want {
				t.Fatalf("%q became %q, want %q", raw, got, want)
			}
		})
	}
}

func TestNormalize_should_leave_what_is_not_a_prefixed_number_alone(t *testing.T) {
	for _, raw := range []string{"beta", "vx", "1.23.b-alpha", "", "versions"} {
		t.Run(raw, func(t *testing.T) {
			got := Normalize(raw)

			if got != raw {
				t.Fatalf("%q became %q", raw, got)
			}
		})
	}
}
