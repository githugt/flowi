// SPDX-License-Identifier: Apache-2.0

package migrate

import (
	"strconv"
	"testing"

	"github.com/githugt/flowi-lib/diag"
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
