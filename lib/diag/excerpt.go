// SPDX-License-Identifier: Apache-2.0

package diag

import (
	"bytes"
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

// tabWidth is how wide a tab is shown in an excerpt.
const tabWidth = 4

// Invisible reports characters that show as nothing (or as a space) but
// change a value; excerpts show them as ⟨U+XXXX⟩.
func Invisible(r rune) bool {
	return slices.Contains([]rune{0x200B, 0x200C, 0x200D, 0x2060, 0xFEFF, 0x00A0}, r)
}

// ExcerptAt is the source line at pos and, when pos has a column, a caret
// line under it. Tabs are widened and invisible characters made visible,
// and the caret accounts for both. Columns count runes from 1. A position
// outside src gives nil.
func ExcerptAt(src []byte, pos Position) []string {
	lines := bytes.Split(src, []byte("\n"))
	if pos.Line < 1 || pos.Line > len(lines) {
		return nil
	}
	var shown, caret strings.Builder
	col := 1
	for _, r := range strings.TrimRight(string(lines[pos.Line-1]), "\r") {
		text := string(r)
		switch {
		case r == '\t':
			text = strings.Repeat(" ", tabWidth)
		case Invisible(r):
			text = fmt.Sprintf("⟨U+%04X⟩", r)
		}
		if col < pos.Column {
			caret.WriteString(strings.Repeat(" ", utf8.RuneCountInString(text)))
		}
		shown.WriteString(text)
		col++
	}
	if pos.Column < 1 {
		return []string{shown.String()}
	}
	return []string{shown.String(), caret.String() + "^"}
}

// PositionAt converts a byte offset in src to a line and rune column.
func PositionAt(src []byte, offset int) Position {
	offset = min(max(offset, 0), len(src))
	before := src[:offset]
	line := bytes.Count(before, []byte("\n")) + 1
	start := bytes.LastIndexByte(before, '\n') + 1
	return Position{Line: line, Column: utf8.RuneCount(before[start:]) + 1}
}

// LineIndex converts many offsets of one source to positions without
// rescanning it: PositionAt in a loop is quadratic on large inputs, and so
// is counting columns from the line start on a long (minified) line. It
// remembers the last position, so increasing offsets cost only the text
// between them. Not safe for concurrent use.
type LineIndex struct {
	src    []byte
	starts []int // byte offset where each line starts
	last   int   // the last offset converted
	lastAt Position
}

// NewLineIndex indexes src's line starts.
func NewLineIndex(src []byte) *LineIndex {
	starts := []int{0}
	for i, b := range src {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	return &LineIndex{src: src, starts: starts, last: -1}
}

// Position is PositionAt(src, offset).
func (x *LineIndex) Position(offset int) Position {
	offset = min(max(offset, 0), len(x.src))
	line := sort.Search(len(x.starts), func(i int) bool { return x.starts[i] > offset }) - 1
	from, col := x.starts[line], 1
	atRuneStart := x.last >= 0 && (x.last == len(x.src) || utf8.RuneStart(x.src[x.last]))
	if atRuneStart && x.last >= from && x.last <= offset && x.lastAt.Line == line+1 {
		from, col = x.last, x.lastAt.Column
	}
	pos := Position{Line: line + 1, Column: col + utf8.RuneCount(x.src[from:offset])}
	x.last, x.lastAt = offset, pos
	return pos
}
