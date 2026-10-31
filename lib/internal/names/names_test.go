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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := names.Valid(tt.name); got != tt.want {
				t.Fatalf("Valid(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
