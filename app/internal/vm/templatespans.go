// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import "strings"

// TemplateSpans splits text into plain parts and {{variables}} (PartVariable
// when known says the expression is defined, else PartUnknownVariable). An
// unclosed {{ stays plain text. With no checker the whole text is one plain
// span. The spans join back to text.
func TemplateSpans(text string, known func(expr string) bool) []URLSpan {
	if text == "" {
		return nil
	}
	if known == nil {
		return []URLSpan{{Text: text, Part: PartText, Pair: -1}}
	}
	var out []URLSpan
	plain := func(s string) {
		if s != "" {
			out = append(out, URLSpan{Text: s, Part: PartText, Pair: -1})
		}
	}
	rest := text
	for {
		open := strings.Index(rest, "{{")
		if open < 0 {
			break
		}
		end := strings.Index(rest[open:], "}}")
		if end < 0 {
			break
		}
		end += open + 2
		plain(rest[:open])
		part := PartUnknownVariable
		if known(strings.TrimSpace(rest[open+2 : end-2])) {
			part = PartVariable
		}
		out = append(out, URLSpan{Text: rest[open:end], Part: part, Pair: -1})
		rest = rest[end:]
	}
	plain(rest)
	return out
}
