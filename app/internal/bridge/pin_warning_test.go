// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/engine"
)

func TestPinWarningText_should_name_the_headers_and_say_why(t *testing.T) {
	// Act
	got := pinWarningText([]string{"Set-Cookie", "X-Auth-Token"})

	// Assert
	if !strings.Contains(got, "Set-Cookie, X-Auth-Token") || !strings.Contains(got, "committed") {
		t.Errorf("got %q", got)
	}
}

func TestPinWarningText_should_be_empty_without_credential_headers(t *testing.T) {
	// Act
	got := pinWarningText(nil)

	// Assert
	if got != "" {
		t.Errorf("got %q", got)
	}
}

func TestPinBodyNote_Given_binary_or_big_bodies_Then_it_says_what_is_kept(t *testing.T) {
	// Before: a binary response pinned as an empty body and a 10 MiB body was committed, with no word.
	tests := []struct {
		name string
		body engine.Body
		want string
	}{
		{"binary", engine.Body{Base64: "AAAA", Size: 3}, "only the headers"},
		{"big text", engine.Body{Text: "x", JSON: map[string]any{}, Size: 10 << 20}, "committed"},
		{"small json", engine.Body{JSON: map[string]any{"a": 1}, Size: 9}, ""},
		{"empty", engine.Body{}, ""},
	}
	for _, tt := range tests {
		got := pinBodyNote(tt.body)
		if (tt.want == "") != (got == "") || !strings.Contains(got, tt.want) {
			t.Errorf("%s: got %q, want it to contain %q", tt.name, got, tt.want)
		}
	}
}
