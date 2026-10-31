// SPDX-License-Identifier: Apache-2.0

package curl

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// token is one shell word and the byte offset where it starts.
type token struct {
	text   string
	offset int
}

// syntaxError is a command that cannot be split into words.
type syntaxError struct {
	offset int
	detail string
}

func (e *syntaxError) Error() string { return e.detail }

// tokenize splits a command into words. Bash rules: '…' is literal, "…"
// keeps backslash escapes of $ ` " \ and newline, $'…' has C escapes,
// backslash-newline continues the line, # starts a comment. With cmd,
// Windows rules ("Copy as cURL (cmd)"): ^ escapes the next character and
// ^-newline continues; then quotes group words and \" is a literal quote.
func tokenize(s string, cmd bool) ([]token, error) {
	if cmd {
		return tokenizeCmd(s)
	}
	t := &bashTokenizer{s: s}
	for t.i < len(s) {
		if err := t.step(); err != nil {
			return nil, err
		}
	}
	t.flush()
	return t.out, nil
}

type bashTokenizer struct {
	s     string
	i     int
	cur   strings.Builder
	have  bool
	start int
	out   []token
}

func (t *bashTokenizer) begin() {
	if !t.have {
		t.have, t.start = true, t.i
	}
}

func (t *bashTokenizer) flush() {
	if t.have {
		t.out = append(t.out, token{text: t.cur.String(), offset: t.start})
		t.cur.Reset()
		t.have = false
	}
}

func (t *bashTokenizer) step() error {
	c := t.s[t.i]
	switch {
	case c == ' ' || c == '\t' || c == '\n' || c == '\r':
		t.flush()
		t.i++
	case c == '\\':
		t.backslash()
	case c == '#' && !t.have:
		for t.i < len(t.s) && t.s[t.i] != '\n' {
			t.i++
		}
	case c == '\'':
		return t.single()
	case c == '"':
		return t.double()
	case c == '$' && strings.HasPrefix(t.s[t.i:], "$'"):
		return t.ansi()
	default:
		t.begin()
		r, size := utf8.DecodeRuneInString(t.s[t.i:])
		t.cur.WriteRune(r)
		t.i += size
	}
	return nil
}

func (t *bashTokenizer) backslash() {
	rest := t.s[t.i+1:]
	switch {
	case strings.HasPrefix(rest, "\r\n"):
		t.i += 3
	case strings.HasPrefix(rest, "\n"):
		t.i += 2
	case rest == "":
		t.begin()
		t.cur.WriteByte('\\')
		t.i++
	default:
		t.begin()
		r, size := utf8.DecodeRuneInString(rest)
		t.cur.WriteRune(r)
		t.i += 1 + size
	}
}

func (t *bashTokenizer) single() error {
	end := strings.IndexByte(t.s[t.i+1:], '\'')
	if end < 0 {
		return &syntaxError{offset: t.i, detail: "the quote ' is never closed"}
	}
	t.begin()
	t.cur.WriteString(t.s[t.i+1 : t.i+1+end])
	t.i += end + 2
	return nil
}

func (t *bashTokenizer) double() error {
	open := t.i
	t.begin()
	for j := t.i + 1; j < len(t.s); j++ {
		switch c := t.s[j]; {
		case c == '"':
			t.i = j + 1
			return nil
		case c == '\\' && j+1 < len(t.s) && strings.IndexByte("$`\"\\\n", t.s[j+1]) >= 0:
			if t.s[j+1] != '\n' {
				t.cur.WriteByte(t.s[j+1])
			}
			j++
		default:
			t.cur.WriteByte(c)
		}
	}
	return &syntaxError{offset: open, detail: `the quote " is never closed`}
}

// ansi reads $'…' with C escapes.
func (t *bashTokenizer) ansi() error {
	open := t.i
	t.begin()
	for j := t.i + 2; j < len(t.s); j++ {
		switch t.s[j] {
		case '\'':
			t.i = j + 1
			return nil
		case '\\':
			if j+1 >= len(t.s) {
				return &syntaxError{offset: open, detail: "the quote $' is never closed"}
			}
			j += t.cEscape(t.s[j+1:])
		default:
			t.cur.WriteByte(t.s[j])
		}
	}
	return &syntaxError{offset: open, detail: "the quote $' is never closed"}
}

// cEscape writes the escape at the start of s (after the backslash) and
// returns how many bytes of s it used.
func (t *bashTokenizer) cEscape(s string) int {
	simple := map[byte]string{'n': "\n", 't': "\t", 'r': "\r", '\\': "\\", '\'': "'", '"': "\"", 'a': "\a", 'b': "\b", 'e': "\x1b", 'f': "\f", 'v': "\v"}
	if out, ok := simple[s[0]]; ok {
		t.cur.WriteString(out)
		return 1
	}
	for prefix, digits := range map[byte]int{'x': 2, 'u': 4, 'U': 8} {
		if s[0] != prefix {
			continue
		}
		n := hexRun(s[1:], digits)
		if n == 0 {
			break
		}
		v, _ := strconv.ParseUint(s[1:1+n], 16, 32) // hexRun guarantees hex digits
		switch {
		case prefix == 'x':
			t.cur.WriteByte(byte(v)) //nolint:gosec // \x takes at most two hex digits
		case v > unicode.MaxRune:
			t.cur.WriteRune(utf8.RuneError)
		default:
			t.cur.WriteRune(rune(v)) //nolint:gosec // v ≤ unicode.MaxRune, checked above
		}
		return 1 + n
	}
	t.cur.WriteByte('\\')
	t.cur.WriteByte(s[0])
	return 1
}

// hexRun counts up to limit leading hex digits.
func hexRun(s string, limit int) int {
	n := 0
	for n < len(s) && n < limit && strings.IndexByte("0123456789abcdefABCDEF", s[n]) >= 0 {
		n++
	}
	return n
}

// tokenizeCmd undoes cmd's ^ escapes, then splits words the way the
// Windows C runtime does.
func tokenizeCmd(s string) ([]token, error) {
	var plain []byte
	var origin []int // plain index → offset in s
	for i := 0; i < len(s); i++ {
		if s[i] != '^' {
			plain, origin = append(plain, s[i]), append(origin, i)
			continue
		}
		switch rest := s[i+1:]; {
		case strings.HasPrefix(rest, "\r\n"):
			i += 2
		case strings.HasPrefix(rest, "\n"):
			i++
		case rest != "":
			plain, origin = append(plain, s[i+1]), append(origin, i)
			i++
		}
	}
	return crtWords(plain, origin)
}

// crtWords splits like CommandLineToArgvW: whitespace separates, "
// toggles quoting, 2n backslashes before " are n backslashes and a quote
// toggle, 2n+1 are n backslashes and a literal ".
func crtWords(p []byte, origin []int) ([]token, error) {
	var out []token
	var cur strings.Builder
	have, quoted, start, quoteAt := false, false, 0, 0
	for i := 0; i < len(p); i++ {
		c := p[i]
		switch {
		case (c == ' ' || c == '\t' || c == '\n' || c == '\r') && !quoted:
			if have {
				out = append(out, token{text: cur.String(), offset: origin[start]})
				cur.Reset()
				have = false
			}
			continue
		case c == '\\':
			n := 0
			for i+n < len(p) && p[i+n] == '\\' {
				n++
			}
			if i+n < len(p) && p[i+n] == '"' {
				cur.WriteString(strings.Repeat("\\", n/2))
				if n%2 == 1 {
					cur.WriteByte('"')
				} else {
					quoted = !quoted
					quoteAt = i + n
				}
				i += n
			} else {
				cur.WriteString(strings.Repeat("\\", n))
				i += n - 1
			}
		case c == '"':
			quoted = !quoted
			quoteAt = i
		default:
			cur.WriteByte(c)
		}
		if !have {
			have, start = true, i
		}
	}
	if quoted {
		return nil, &syntaxError{offset: origin[quoteAt], detail: `the quote " is never closed`}
	}
	if have {
		out = append(out, token{text: cur.String(), offset: origin[start]})
	}
	return out, nil
}
