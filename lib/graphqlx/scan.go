// SPDX-License-Identifier: Apache-2.0

package graphqlx

import "strings"

// scanner walks a document one token at a time, tracking how deep in brackets it is. Tokens are names, numbers and
// single punctuation marks; strings come back as one token each, so their content is never mistaken for syntax.
type scanner struct {
	src   string
	pos   int
	depth int // open { ( [ after the current token
	// top is true when the current token started outside every bracket; wasHeader is header as the token found it.
	top, wasHeader bool
	// header is true from an operation or fragment keyword to the "{" that opens its selection set.
	header     bool
	kind       Kind // the kind of the header being read, "" for a fragment
	nameWanted bool
}

// next returns the next token, or false at the end. It keeps depth, and header when a top-level selection set closes.
func (s *scanner) next() (string, bool) {
	s.skipBlank()
	if s.pos >= len(s.src) {
		return "", false
	}
	s.top, s.wasHeader = s.depth == 0, s.header
	start := s.pos
	c := s.src[s.pos]
	switch {
	case c == '"':
		s.skipString()
		s.pos = min(s.pos, len(s.src)) // an escape at the very end steps past it
		return s.src[start:s.pos], true
	case c == '{' || c == '(' || c == '[':
		s.pos++
		if s.depth == 0 && c == '{' {
			s.header = false // the header ends here
		}
		s.depth++
		return s.src[start:s.pos], true
	case c == '}' || c == ')' || c == ']':
		s.pos++
		if s.depth > 0 {
			s.depth--
		}
		return s.src[start:s.pos], true
	case isWordByte(c):
		for s.pos < len(s.src) && isWordByte(s.src[s.pos]) {
			s.pos++
		}
		return s.src[start:s.pos], true
	}
	s.pos++
	return s.src[start:s.pos], true
}

// skipBlank skips white space, commas (insignificant in GraphQL) and # comments.
func (s *scanner) skipBlank() {
	for s.pos < len(s.src) {
		switch c := s.src[s.pos]; {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ',':
			s.pos++
		case c == '#':
			for s.pos < len(s.src) && s.src[s.pos] != '\n' && s.src[s.pos] != '\r' {
				s.pos++
			}
		case strings.HasPrefix(s.src[s.pos:], "\xef\xbb\xbf"): // a byte order mark
			s.pos += 3
		default:
			return
		}
	}
}

// skipString moves past a string or block string starting at pos.
func (s *scanner) skipString() {
	if strings.HasPrefix(s.src[s.pos:], `"""`) {
		s.pos += 3
		for s.pos < len(s.src) {
			switch {
			case strings.HasPrefix(s.src[s.pos:], `\"""`):
				s.pos += 4
			case strings.HasPrefix(s.src[s.pos:], `"""`):
				s.pos += 3
				return
			default:
				s.pos++
			}
		}
		return
	}
	s.pos++
	for s.pos < len(s.src) {
		switch s.src[s.pos] {
		case '\\':
			s.pos += 2
		case '"':
			s.pos++
			return
		case '\n':
			return // an unterminated string ends at the line; the server will say what is wrong
		default:
			s.pos++
		}
	}
}

func isWordByte(c byte) bool {
	return c == '_' || c == '.' || c == '$' || c == '@' || c == '-' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}
