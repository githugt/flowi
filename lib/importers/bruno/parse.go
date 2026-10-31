// SPDX-License-Identifier: Apache-2.0

package bruno

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/githugt/flowi/lib/internal/textenc"
)

// block is one top-level block of a .bru file: a dictionary of entries
// (meta { … }), text (body:json { … }) or a list (vars:secret [ … ]).
type block struct {
	name    string
	line    int
	entries []entry
	text    string
	list    []string
}

// entry is one "name: value" line; ~ marks it disabled.
type entry struct {
	key, value string
	disabled   bool
	line, col  int
}

// syntaxError is a .bru file that cannot be read, at a line and column.
type syntaxError struct {
	line, col int
	detail    string
}

func (e *syntaxError) Error() string { return fmt.Sprintf("%d:%d: %s", e.line, e.col, e.detail) }

var header = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_:-]*)\s*([{\[])\s*$`)

// textBlock reports blocks whose body is text, closed by a "}" alone at
// the start of a line, with Bruno's two-space indent removed.
func textBlock(name string) bool {
	if slices.Contains([]string{"body:form-urlencoded", "body:multipart-form"}, name) {
		return false
	}
	return name == "docs" || name == "tests" || strings.HasPrefix(name, "script:") || strings.HasPrefix(name, "body:")
}

// parse reads a .bru file.
func parse(src []byte) ([]block, error) {
	lines := strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")
	var out []block
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		m := header.FindStringSubmatch(strings.TrimSpace(lines[i]))
		if m == nil {
			return nil, &syntaxError{line: i + 1, col: indent(lines[i]) + 1, detail: "expected a block such as meta { … }"}
		}
		b := block{name: m[1], line: i + 1}
		var err error
		switch {
		case m[2] == "[":
			i, err = readList(lines, i, &b)
		case textBlock(b.name):
			i, err = readText(lines, i, &b)
		default:
			i, err = readEntries(lines, i, &b)
		}
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}

func indent(line string) int {
	return utf8.RuneCountInString(line) - utf8.RuneCountInString(strings.TrimLeft(line, " \t"))
}

func unclosed(b *block) error {
	return &syntaxError{line: b.line, col: 1, detail: fmt.Sprintf("block %s is never closed", b.name)}
}

// readList reads items separated by commas or lines until "]".
func readList(lines []string, start int, b *block) (int, error) {
	for i := start + 1; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if t == "]" {
			return i, nil
		}
		for _, item := range strings.Split(t, ",") {
			if item = strings.TrimSpace(item); item != "" {
				b.list = append(b.list, item)
			}
		}
	}
	return 0, unclosed(b)
}

// readText reads lines until a "}" at the start of a line.
func readText(lines []string, start int, b *block) (int, error) {
	var body []string
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], " \t") == "}" {
			b.text = strings.Join(body, "\n")
			return i, nil
		}
		body = append(body, strings.TrimPrefix(lines[i], "  "))
	}
	return 0, unclosed(b)
}

// readEntries reads "name: value" lines (names have no spaces) until "}".
// A value of three single quotes starts a multi-line value that ends at a
// line holding only three single quotes.
func readEntries(lines []string, start int, b *block) (int, error) {
	for i := start + 1; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		switch t {
		case "}":
			return i, nil
		case "":
			continue
		}
		e := entry{line: i + 1, col: indent(lines[i]) + 1}
		t, e.disabled = strings.CutPrefix(t, "~")
		key, value, ok := strings.Cut(t, ":")
		if !ok || strings.TrimSpace(key) == "" || strings.ContainsAny(strings.TrimSpace(key), " \t") {
			return 0, &syntaxError{line: e.line, col: e.col, detail: fmt.Sprintf("expected name: value in block %s", b.name)}
		}
		e.key, e.value = strings.TrimSpace(key), strings.TrimSpace(value)
		if e.value == "'''" {
			end, text, err := readMultiline(lines, i, b)
			if err != nil {
				return 0, err
			}
			i, e.value = end, text
		}
		b.entries = append(b.entries, e)
	}
	return 0, unclosed(b)
}

func readMultiline(lines []string, start int, b *block) (int, string, error) {
	var body []string
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "'''" {
			return i, strings.Join(body, "\n"), nil
		}
		body = append(body, strings.TrimPrefix(lines[i], "    "))
	}
	return 0, "", unclosed(b)
}

// find returns the first block named name.
func find(blocks []block, name string) (block, bool) {
	i := slices.IndexFunc(blocks, func(b block) bool { return b.name == name })
	if i < 0 {
		return block{}, false
	}
	return blocks[i], true
}

// value returns an enabled entry's value.
func (b block) value(key string) string {
	for _, e := range b.entries {
		if e.key == key && !e.disabled {
			return e.value
		}
	}
	return ""
}

// enabled returns the block's enabled entries.
func (b block) enabled() []entry {
	return slices.DeleteFunc(slices.Clone(b.entries), func(e entry) bool { return e.disabled })
}

// trimBOM drops a byte order mark and converts UTF-16 text.
func trimBOM(src []byte) []byte { return textenc.ToUTF8(src) }
