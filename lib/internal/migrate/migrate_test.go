// SPDX-License-Identifier: Apache-2.0

package migrate

import (
	"strconv"
	"testing"

	"github.com/githugt/flowi/lib/diag"
)

func TestCheck(t *testing.T) {
	tests := []struct {
		format int
		want   diag.Code
	}{
		{0, diag.CodeFormatMissing},
		{1, ""},
		{2, diag.CodeFormatTooNew},
	}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.format), func(t *testing.T) {
			got := Check(tt.format)

			if got.Code != tt.want {
				t.Fatalf("got %q, want %q", got.Code, tt.want)
			}
		})
	}
}

func TestCheck_too_new_should_name_both_versions(t *testing.T) {
	got := Check(7)

	if got.Args["got"] != "7" || got.Args["max"] != "1" {
		t.Fatalf("args %v", got.Args)
	}
}

func TestPlan_should_return_no_steps_for_current_format(t *testing.T) {
	steps, err := Plan(CurrentFormat)

	if err != nil || len(steps) != 0 {
		t.Fatalf("got %v, %v", steps, err)
	}
}

func TestPlan_should_error_when_step_missing(t *testing.T) {
	steps := []Step{{From: 1, Describe: "1→2"}, {From: 3, Describe: "3→4"}}

	_, err := planWith(steps, 1, 4)

	if err == nil {
		t.Fatal("expected an error for the missing 2→3 step")
	}
}

func TestPlan_should_return_steps_in_order(t *testing.T) {
	steps := []Step{{From: 2, Describe: "2→3"}, {From: 1, Describe: "1→2"}}

	got, err := planWith(steps, 1, 3)

	if err != nil || len(got) != 2 || got[0].From != 1 || got[1].From != 2 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestCheckVersion(t *testing.T) {
	// Before: a collection written for a newer Flowi failed in an older CI CLI with an unrelated error.
	tests := []struct {
		name, min, have string
		wantNeedsNewer  bool
	}{
		{"older release", "1.4.0", "1.3.9", true},
		{"same release", "1.4.0", "1.4.0", false},
		{"newer release", "1.4.0", "1.10.0", false},
		{"numbers compare as numbers", "1.10.0", "1.9.0", true},
		{"missing parts are zero", "v2", "1.99.99", true},
		{"a pre-release is before its release", "1.4.0", "1.4.0-rc.1", true},
		{"a release is after its pre-release", "1.4.0-rc.1", "1.4.0", false},
		{"no minimum", "", "0.1.0", false},
		{"a development build reads everything", "9.0.0", "dev", false},
		{"a minimum that is not a version is ignored", "soon", "1.0.0", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckVersion(tt.min, tt.have)

			if (got.Code == diag.CodeFormatNeedsNewer) != tt.wantNeedsNewer {
				t.Fatalf("got %q", got.Code)
			}
		})
	}
}

func TestCheckVersion_should_name_both_releases(t *testing.T) {
	got := CheckVersion("1.4.0", "1.3.0")

	if got.Args["min"] != "1.4.0" || got.Args["have"] != "1.3.0" {
		t.Fatalf("args %v", got.Args)
	}
}
