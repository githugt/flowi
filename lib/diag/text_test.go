// SPDX-License-Identifier: Apache-2.0

package diag_test

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
)

func TestText_should_keep_the_hint_after_the_message(t *testing.T) {
	// Before: one-line callers took only the message of English().Message and dropped the hint, the part that says what to do.
	for _, code := range diag.Codes() {
		d := diag.New(code, diag.Location{}, nil)
		msg, hint := diag.English().Message(d)
		if hint == "" {
			continue
		}
		got := diag.Text(d)
		if !strings.HasPrefix(got, msg) || !strings.HasSuffix(got, "("+hint+")") {
			t.Fatalf("%s: Text %q lost the hint %q", code, got, hint)
		}
		return // one code with a hint is enough to show the rule
	}
	t.Fatal("no code has a hint to test with")
}

func TestText_Given_a_code_without_hint_Then_it_is_the_message(t *testing.T) {
	d := diag.New(diag.Code("FLW-NOPE-999"), diag.Location{}, nil)

	if got := diag.Text(d); got != "FLW-NOPE-999" {
		t.Fatalf("got %q", got)
	}
}
