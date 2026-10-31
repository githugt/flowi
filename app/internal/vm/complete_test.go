// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"testing"

	flowi "github.com/githugt/flowi-lib"

	"github.com/githugt/flowi/internal/vm"
)

func TestComplete_should_match_the_token_being_typed(t *testing.T) {
	all := []flowi.Completion{{Text: "nodes.login.token"}, {Text: "nodes.login.status"}, {Text: "vars.token"}, {Text: "inputs.email"}}
	cases := []struct {
		text   string
		cursor int
		want   []string
		start  int
	}{
		{"Bearer {{nodes.login.t", 22, []string{"nodes.login.token"}, 9},
		{"{{tok", 5, []string{"nodes.login.token", "vars.token"}, 2},
		{"{{vars.token}} x", 16, nil, 0}, // the template is closed: nothing to complete
		{"plain text", 10, nil, 0},
		{"{{", 2, []string{"inputs.email", "nodes.login.status", "nodes.login.token", "vars.token"}, 2},
	}
	for _, tc := range cases {
		got, start := vm.Complete(tc.text, tc.cursor, all)
		var texts []string
		for _, c := range got {
			texts = append(texts, c.Text)
		}
		if len(texts) != len(tc.want) || (len(texts) > 0 && start != tc.start) {
			t.Errorf("Complete(%q) = %v at %d, want %v at %d", tc.text, texts, start, tc.want, tc.start)
			continue
		}
		for i := range texts {
			if texts[i] != tc.want[i] {
				t.Errorf("Complete(%q) = %v, want %v", tc.text, texts, tc.want)
			}
		}
	}
}
