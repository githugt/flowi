// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"testing"

	"github.com/githugt/flowi/internal/vm"
)

// Behaviour change: the Wails window offered "pokeapi.co/api/v2/pokemon/ditto",
// which saved the request in a folder per URL segment. The suggestion is one
// dashed name.
func TestDefaultSavePath_should_make_one_name_of_an_untitled_requests_url(t *testing.T) {
	tests := []struct{ name, url, want string }{
		{"untitled", "https://pokeapi.co/api/v2/pokemon/ditto", "pokeapi-co-api-v2-pokemon-ditto"},
		{"untitled-2", "http://x.io/a?b=1", "x-io-a"},
		{"untitled", "", "untitled"},
		{"login", "https://x.io/a", "login"},
	}
	for _, tt := range tests {
		if got := vm.DefaultSavePath(tt.name, tt.url); got != tt.want {
			t.Errorf("DefaultSavePath(%q, %q) = %q, want %q", tt.name, tt.url, got, tt.want)
		}
	}
}
