// SPDX-License-Identifier: Apache-2.0

package diag

import (
	"slices"
	"strings"
	"testing"
)

func TestExcerptAt_should_show_the_line_and_a_caret_under_the_column(t *testing.T) {
	src := []byte("first\nGET https://x\u200b.example/ü\tend\n")
	cases := []struct {
		name string
		pos  Position
		want []string
	}{
		{"plain", Position{Line: 1, Column: 3}, []string{"first", "  ^"}},
		// "GET https://x" is 13 wide, ⟨U+200B⟩ 8, ".example" 8: the / is
		// at display column 30.
		{"invisible shown and counted", Position{Line: 2, Column: 23}, []string{
			"GET https://x⟨U+200B⟩.example/ü    end", strings.Repeat(" ", 29) + "^"}},
		// "/ü" adds 2 and the tab 4: the e after it is at display column 36.
		{"tab widened before the caret", Position{Line: 2, Column: 26}, []string{
			"GET https://x⟨U+200B⟩.example/ü    end", strings.Repeat(" ", 35) + "^"}},
		{"no column", Position{Line: 1}, []string{"first"}},
		{"line out of range", Position{Line: 9, Column: 1}, nil},
		{"no position", Position{}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExcerptAt(src, tc.pos); !slices.Equal(got, tc.want) {
				t.Fatalf("got  %q\nwant %q", got, tc.want)
			}
		})
	}
}

func TestPositionAt_should_count_lines_and_runes(t *testing.T) {
	src := []byte("ab\nüx\u200by")

	got := PositionAt(src, len("ab\nüx\u200b"))

	if got != (Position{Line: 2, Column: 4}) {
		t.Fatalf("got %+v", got)
	}
}

func TestLineIndex_should_match_PositionAt(t *testing.T) {
	src := []byte("ab\nüx\u200by\n\nz")
	idx := NewLineIndex(src)
	for off := 0; off <= len(src); off++ {
		if got, want := idx.Position(off), PositionAt(src, off); got != want {
			t.Fatalf("offset %d: %+v, want %+v", off, got, want)
		}
	}
}
