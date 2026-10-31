// SPDX-License-Identifier: Apache-2.0

package yamlpath

import "testing"

func TestKey(t *testing.T) {
	tests := []struct{ parent, key, want string }{
		{"$", "nodes", "$.nodes"},
		{"$.versions", "1", "$.versions.1"},
		{"$.versions", "1.0", "$.versions.'1.0'"},
		{"$.headers", "X Trace", "$.headers.'X Trace'"},
		{"$.vars", "it's", `$.vars.'it\'s'`},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := Key(tt.parent, tt.key); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIndex(t *testing.T) {
	if got := Index("$.nodes", 3); got != "$.nodes[3]" {
		t.Fatalf("got %q", got)
	}
}
