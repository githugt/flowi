// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"strings"
	"testing"
)

func TestReportText_should_keep_one_line_and_cut_long_text(t *testing.T) {
	// Before: nothing the window caught reached the log.
	got := reportText("TypeError: x is undefined\n  at f (a.js:1)\r\n  at g")

	if strings.ContainsAny(got, "\r\n") || !strings.HasPrefix(got, "TypeError: x is undefined | ") {
		t.Fatalf("got %q", got)
	}
	long := reportText(strings.Repeat("é", maxReportText))
	if len(long) > maxReportText+len("…") || !strings.HasSuffix(long, "…") {
		t.Fatalf("not cut: %d bytes", len(long))
	}
}

func TestReportKind_should_only_allow_the_two_kinds(t *testing.T) {
	if reportKind("rejection") != "rejection" || reportKind("anything\nelse") != "error" {
		t.Fatal("kind not limited")
	}
}
