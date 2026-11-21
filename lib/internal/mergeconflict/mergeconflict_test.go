// SPDX-License-Identifier: Apache-2.0

package mergeconflict

import "testing"

func TestFind_should_report_the_line_of_an_unresolved_conflict_and_nothing_else(t *testing.T) {
	tests := []struct {
		name string
		in   string
		line int
	}{
		{"none", "a: 1\nb: 2\n", 0},
		{"plain_conflict", "a: 1\n<<<<<<< HEAD\nb: 2\n=======\nb: 3\n>>>>>>> topic\n", 2},
		{"diff3_style", "<<<<<<< ours\nx\n||||||| base\ny\n=======\nz\n>>>>>>> theirs\n", 1},
		{"windows_line_ends", "a: 1\r\n<<<<<<< HEAD\r\nb\r\n=======\r\nc\r\n>>>>>>> t\r\n", 2},
		{"start_without_the_rest", "text: |\n  <<<<<<< not a marker\n<<<<<<< alone\n", 0},
		{"indented_markers_are_text", "t: |\n  <<<<<<< a\n  =======\n  >>>>>>> b\n", 0},
	}
	for _, tt := range tests {
		got, ok := Find([]byte(tt.in))

		if got != tt.line || ok != (tt.line > 0) {
			t.Errorf("%s: got %d, %v, want %d", tt.name, got, ok, tt.line)
		}
	}
}
