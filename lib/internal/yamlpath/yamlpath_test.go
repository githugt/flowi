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

func TestParent_Given_paths_When_the_last_step_is_dropped_Then_the_enclosing_place_remains(t *testing.T) {
	cases := map[string]string{
		"$.a.b[1]":                 "$.a.b",
		"$.a.b":                    "$.a",
		"$.a":                      "$",
		"$":                        "",
		"$.paths.'/pets/{id}'.get": "$.paths.'/pets/{id}'",
		"$.paths.'/v1.0/pets'":     "$.paths",
		"$.x.'it\\'s'":             "$.x",
		"$.servers[0]":             "$.servers",
	}
	for in, want := range cases {
		if got := Parent(in); got != want {
			t.Errorf("Parent(%q) = %q, want %q", in, got, want)
		}
	}
}
