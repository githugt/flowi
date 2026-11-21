// SPDX-License-Identifier: Apache-2.0

package names_test

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/internal/names"
)

func TestValid_should_refuse_separators_dots_and_long_names(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"ok-1.json", true},
		{"200", true},
		{"a_b", true},
		{strings.Repeat("a", 64), true},
		{strings.Repeat("a", 65), false},
		{"", false},
		{"../x", false},
		{"a/b", false},
		{`a\b`, false},
		{"/etc/x", false},
		{".hidden", false},
		{"..", false},
		{"a b", false},
		{"Café", true},
		{"日本語-1", true},
		{"e\u0301", false},  // not in normal form C: macOS would store it as another spelling
		{"a\u202eb", false}, // a bidi control
		{"a\x00b", false},
		{"a\tb", false},
		{"trailing.", false},
		{"CON", false},
		{"nul.txt", false},
		{"com1", false},
		{"com10", true},
		{"console", true},
		{strings.Repeat("日", 64), true},
		{strings.Repeat("日", 70), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := names.Valid(tt.name); got != tt.want {
				t.Fatalf("Valid(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestNormalize_should_give_the_spelling_Valid_accepts(t *testing.T) {
	if got := names.Normalize("e\u0301"); got != "\u00e9" || !names.Valid(got) {
		t.Fatalf("%q", got)
	}
}
