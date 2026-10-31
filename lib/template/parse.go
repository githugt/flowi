// SPDX-License-Identifier: Apache-2.0

package template

import (
	"fmt"
	"strings"
)

const (
	openDelim  = "{{"
	closeDelim = "}}"
)

// Template is a parsed template. The zero value renders "".
type Template struct {
	segments []segment
}

// segment is literal text, or an expression when expr is set.
type segment struct {
	text string
	expr *expression
}

// SyntaxError reports a template that cannot be parsed. Offset is the byte
// offset in the template source of the placeholder at fault.
type SyntaxError struct {
	Offset int
	Detail string
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("template syntax error at offset %d: %s", e.Offset, e.Detail)
}

// Parse splits src into text and {{…}} expressions and compiles each
// expression. A "}}" outside a placeholder is ordinary text (JSON bodies are
// full of them); a "{{" without a closing "}}" is an error.
func Parse(src string) (Template, error) {
	var t Template
	rest, offset := src, 0
	for rest != "" {
		start := strings.Index(rest, openDelim)
		if start < 0 {
			t.segments = append(t.segments, segment{text: rest})
			break
		}
		if start > 0 {
			t.segments = append(t.segments, segment{text: rest[:start]})
		}
		body, consumed, err := placeholder(rest[start:], offset+start)
		if err != nil {
			return Template{}, err
		}
		t.segments = append(t.segments, segment{expr: body})
		rest = rest[start+consumed:]
		offset += start + consumed
	}
	return t, nil
}

// placeholder compiles the expression at the start of s (which begins with
// "{{") and reports how many bytes it consumed.
func placeholder(s string, offset int) (*expression, int, error) {
	end := closingDelim(s, len(openDelim))
	if end < 0 {
		return nil, 0, &SyntaxError{Offset: offset, Detail: `missing "}}"`}
	}
	src := strings.TrimSpace(s[len(openDelim):end])
	if src == "" {
		return nil, 0, &SyntaxError{Offset: offset, Detail: "empty expression"}
	}
	e, err := compile(src)
	if err != nil {
		return nil, 0, &SyntaxError{Offset: offset, Detail: err.Error()}
	}
	return e, end + len(closeDelim), nil
}

// closingDelim finds the "}}" that ends a placeholder, skipping string
// literals so {{ x ?? "}}" }} works. It returns -1 when there is none.
func closingDelim(s string, from int) int {
	for i := from; i < len(s); i++ {
		if isQuote(s[i]) {
			i = skipString(s, i)
			continue
		}
		if strings.HasPrefix(s[i:], closeDelim) {
			return i
		}
	}
	return -1
}

func isQuote(c byte) bool { return c == '"' || c == '\'' || c == '`' }

// skipString returns the index of the quote that closes the string literal
// opening at s[i], or len(s)-1 when it never closes (the expression
// compiler then reports it).
func skipString(s string, i int) int {
	quote := s[i]
	for j := i + 1; j < len(s); j++ {
		switch {
		case s[j] == '\\' && quote != '`':
			j++
		case s[j] == quote:
			return j
		}
	}
	return len(s) - 1
}

// IsStatic reports whether the template has no expressions.
func (t Template) IsStatic() bool {
	for _, s := range t.segments {
		if s.expr != nil {
			return false
		}
	}
	return true
}

// References lists the variables the template reads, as dotted paths
// ("env.base_url", "email"), in order of appearance.
func (t Template) References() []string {
	var refs []string
	for _, s := range t.segments {
		if s.expr == nil {
			continue
		}
		for _, c := range s.expr.chains {
			refs = append(refs, c.dotted())
		}
	}
	return refs
}
