// SPDX-License-Identifier: Apache-2.0

package template

import (
	"regexp"
	"strings"
)

// durationLiteral matches Go-style durations such as 2s, 800ms, 1m30s.
// Alternatives are tried in order, so "ms" wins over "m".
var durationLiteral = regexp.MustCompile(`^(?:\d+(?:\.\d+)?(?:ns|us|µs|ms|s|m|h))+`)

// hyphenatedName matches ns.some-name: a namespace followed by a name with
// hyphens between its words, digits included (env.count-1 is the variable
// "count-1"). Only a hyphen with spaces around it is a minus (env.a - 1).
var hyphenatedName = regexp.MustCompile(`^(env|secret|var|vars|with|inputs)\.([A-Za-z_][A-Za-z0-9_]*(?:-[A-Za-z0-9_]+)+)`)

// rewrite turns the spec's notation into plain expr syntax, leaving string
// literals untouched:
//
//	$first-name     → __gen("first-name")
//	$int(1, 100)    → __gen("int", 1, 100)
//	800ms           → duration("800ms")
//	secret.my-key   → secret["my-key"]
func rewrite(src string) string {
	var out strings.Builder
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case isQuote(c):
			end := skipString(src, i)
			out.WriteString(src[i : end+1])
			i = end
		case !followsIdent(src, i) && (i == 0 || src[i-1] != '.') && hyphenatedName.MatchString(src[i:]):
			m := hyphenatedName.FindStringSubmatch(src[i:])
			out.WriteString(m[1] + `["` + m[2] + `"]`)
			i += len(m[0]) - 1
		case c == '$' && startsIdent(src, i+1) && !followsIdent(src, i):
			i = rewriteGenerator(src, i, &out)
		case isDigit(c) && !followsIdent(src, i) && (i == 0 || src[i-1] != '.'):
			i = rewriteDuration(src, i, &out)
		default:
			out.WriteByte(c)
		}
	}
	return out.String()
}

// rewriteGenerator writes the call for the generator at src[i] ('$') and
// returns the index of the last byte it consumed.
func rewriteGenerator(src string, i int, out *strings.Builder) int {
	j := i + 1
	for j < len(src) && (isIdentByte(src[j]) || src[j] == '-') {
		j++
	}
	for src[j-1] == '-' { // a trailing hyphen is a minus sign, not part of the name
		j--
	}
	name := src[i+1 : j]
	if j >= len(src) || src[j] != '(' {
		out.WriteString(`__gen("` + name + `")`)
		return j - 1
	}
	k := j + 1
	for k < len(src) && src[k] == ' ' {
		k++
	}
	if k < len(src) && src[k] == ')' {
		out.WriteString(`__gen("` + name + `")`)
		return k
	}
	out.WriteString(`__gen("` + name + `", `)
	return j
}

// rewriteDuration writes duration("…") when a duration literal starts at
// src[i], or the digit itself otherwise. It returns the index of the last
// byte consumed.
func rewriteDuration(src string, i int, out *strings.Builder) int {
	lit := durationLiteral.FindString(src[i:])
	end := i + len(lit)
	if lit == "" || (end < len(src) && isIdentByte(src[end])) {
		out.WriteByte(src[i])
		return i
	}
	out.WriteString(`duration("` + lit + `")`)
	return end - 1
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isIdentByte(c byte) bool {
	return c == '_' || isDigit(c) || (c|0x20 >= 'a' && c|0x20 <= 'z')
}

func startsIdent(s string, i int) bool {
	return i < len(s) && (s[i] == '_' || (s[i]|0x20 >= 'a' && s[i]|0x20 <= 'z'))
}

// followsIdent reports whether s[i] continues an identifier or number, so
// "x1s" and "$a" inside "b$a" are left alone.
func followsIdent(s string, i int) bool {
	return i > 0 && (isIdentByte(s[i-1]) || s[i-1] == '$')
}
