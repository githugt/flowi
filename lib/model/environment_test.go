// SPDX-License-Identifier: Apache-2.0

package model

import "testing"

func TestDefaultEnvColour(t *testing.T) {
	tests := []struct{ name, want string }{
		{"prod", "red"},
		{"Production", "red"},
		{"PROD-eu", "red"},
		{"staging", ""},
		{"preprod", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DefaultEnvColour(tt.name)

			if got != tt.want {
				t.Fatalf("DefaultEnvColour(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
