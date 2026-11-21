// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"strings"
	"testing"
	"time"
)

// Before: a check being written could only be tried by sending the request again, which for a POST that
// creates data made another record.
func TestEvalChecks_should_judge_each_check_on_the_response_already_received(t *testing.T) {
	rec := Record{
		Status:  201,
		Header:  []Header{{Name: "Content-Type", Value: "application/json"}},
		Body:    Body{Text: `{"id": 7, "name": "Ann"}`, JSON: map[string]any{"id": 7, "name": "Ann"}},
		Timings: Timings{Total: 120 * time.Millisecond},
	}

	got := EvalChecks(rec, []string{
		"status == 201",
		"response.body.name == 'Bob'",
		`response.headers["content-type"] contains "json"`,
		"response.time < 800ms",
		"vars.token != empty",
	})

	want := []string{CheckPassed, CheckFailed, CheckPassed, CheckPassed, CheckUnknown}
	for i, w := range want {
		if got[i].Verdict != w {
			t.Errorf("%q: %s (%s), want %s", got[i].Line, got[i].Verdict, got[i].Note, w)
		}
	}
	if !strings.Contains(got[1].Note, "response.body.name = ") || got[4].Note == "" {
		t.Errorf("notes: %q / %q", got[1].Note, got[4].Note)
	}
}

func TestEvalChecks_Given_no_checks_Then_the_list_is_empty_not_nil(t *testing.T) {
	if got := EvalChecks(Record{}, nil); got == nil || len(got) != 0 {
		t.Fatalf("%#v", got)
	}
}
