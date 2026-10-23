// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/template"
)

// URLPart is what a stretch of a URL is, for colouring.
type URLPart int

// URL parts.
const (
	PartText URLPart = iota
	PartScheme
	PartSeparator // ://
	PartSubdomain // www.
	PartHost      // domain and top-level domain
	PartPort      // :8080
	PartPath
	PartVariable
	PartUnknownVariable
	PartQueryMark // ? and &
	PartKey
	PartEquals
	PartValue
	PartFragmentMark // #
	PartFragment
)

func (p URLPart) String() string {
	return [...]string{"text", "scheme", "separator", "subdomain", "host", "port", "path", "variable", "unknown-variable",
		"query-mark", "key", "equals", "value", "fragment-mark", "fragment"}[p]
}

// URLSpan is a run of URL text of one part. Pair numbers the key=value
// pair a key, equals, value or variable inside one belongs to (from 0), or
// is -1.
type URLSpan struct {
	Text string
	Part URLPart
	Pair int
}

// varMark stands for a byte of a {{variable}} while the structure is read:
// delimiters inside a variable are not URL delimiters.
const varMark = 0x01

// TokenizeURL splits a URL as typed (it may hold {{variables}}) into
// coloured parts. known says whether a variable's expression ("env.x",
// "$uuid-v4") is defined; unknown ones become PartUnknownVariable. Any
// input, however malformed, yields spans that join back to it.
func TokenizeURL(url string, known func(expr string) bool) []URLSpan {
	if url == "" {
		return nil
	}
	t := &urlTokenizer{src: url, parts: make([]URLPart, len(url)), pairs: make([]int, len(url)), ids: make([]int, len(url))}
	for i := range t.pairs {
		t.pairs[i] = -1
	}
	t.masked, t.vars = maskVariables(url)
	t.classify()
	t.overlayVariables(known)
	return t.spans()
}

type urlTokenizer struct {
	src, masked string
	vars        [][2]int // [start, end) of each {{variable}}
	parts       []URLPart
	pairs       []int
	ids         []int // variable number + 1 per byte, 0 outside variables
}

// maskVariables replaces every {{…}} with varMark bytes and lists their
// ranges. An unclosed {{ is plain text.
func maskVariables(s string) (string, [][2]int) {
	b := []byte(s)
	var vars [][2]int
	for i := 0; i+1 < len(s); {
		if s[i] != '{' || s[i+1] != '{' {
			i++
			continue
		}
		end := strings.Index(s[i+2:], "}}")
		if end < 0 {
			break
		}
		stop := i + 2 + end + 2
		for j := i; j < stop; j++ {
			b[j] = varMark
		}
		vars = append(vars, [2]int{i, stop})
		i = stop
	}
	return string(b), vars
}

func (t *urlTokenizer) fill(from, to int, part URLPart) {
	for i := from; i < to && i < len(t.parts); i++ {
		t.parts[i] = part
	}
}

func (t *urlTokenizer) classify() {
	m := t.masked
	i := 0
	hasScheme := false
	if k := schemeEnd(m); k > 0 {
		t.fill(0, k, PartScheme)
		t.fill(k, k+3, PartSeparator)
		i, hasScheme = k+3, true
	}
	if hasScheme || looksLikeHost(m[i:]) {
		end := i + strings.IndexAny(m[i:]+"/", "/?#")
		t.authority(i, end)
		i = end
	}
	if i < len(m) && m[i] != '?' && m[i] != '#' {
		end := i + strings.IndexAny(m[i:]+"?", "?#")
		t.fill(i, end, PartPath)
		i = end
	}
	if i < len(m) && m[i] == '?' {
		end := i + strings.IndexByte(m[i:]+"#", '#')
		t.fill(i, i+1, PartQueryMark)
		t.pairsIn(i+1, end)
		i = end
	}
	if i < len(m) && m[i] == '#' {
		t.fill(i, i+1, PartFragmentMark)
		if strings.Contains(m[i+1:], "=") {
			t.pairsIn(i+1, len(m))
		} else {
			t.fill(i+1, len(m), PartFragment)
		}
	}
}

// schemeEnd is the index of "://" when the URL starts with a scheme, else 0.
func schemeEnd(m string) int {
	k := strings.Index(m, "://")
	if k <= 0 || !isLetter(m[0]) {
		return 0
	}
	for i := 1; i < k; i++ {
		if !isLetter(m[i]) && !strings.ContainsRune("0123456789+-.", rune(m[i])) {
			return 0
		}
	}
	return k
}

func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

// looksLikeHost says whether the part of a scheme-less URL before its first
// "/", "?" or "#" is a host: it has a dot or a port, or is localhost, or is
// only a variable.
func looksLikeHost(rest string) bool {
	seg := rest[:strings.IndexAny(rest+"/", "/?#")]
	switch {
	case seg == "":
		return false
	case strings.HasPrefix(seg, string([]byte{varMark})) && len(strings.Trim(seg, string([]byte{varMark}))) == 0, strings.EqualFold(seg, "localhost"), strings.HasPrefix(strings.ToLower(seg), "localhost:"):
		return true
	}
	return strings.Contains(seg, ".") || strings.Contains(seg, ":")
}

// authority colours [from, to): optional user info and host, subdomain
// "www.", host and port.
func (t *urlTokenizer) authority(from, to int) {
	hostFrom := from
	if at := strings.LastIndexByte(t.masked[from:to], '@'); at >= 0 {
		hostFrom = from + at + 1
		t.fill(from, hostFrom, PartHost)
	}
	hostTo := to
	if c := strings.LastIndexByte(t.masked[hostFrom:to], ':'); c >= 0 && onlyDigitsOrVars(t.masked[hostFrom+c+1:to]) {
		hostTo = hostFrom + c
		t.fill(hostTo, to, PartPort)
	}
	if strings.HasPrefix(strings.ToLower(t.masked[hostFrom:hostTo]), "www.") {
		t.fill(hostFrom, hostFrom+4, PartSubdomain)
		hostFrom += 4
	}
	t.fill(hostFrom, hostTo, PartHost)
}

func onlyDigitsOrVars(s string) bool {
	for i := 0; i < len(s); i++ {
		if (s[i] < '0' || s[i] > '9') && s[i] != varMark {
			return false
		}
	}
	return true
}

// pairsIn colours [from, to) as key=value pairs separated by "&".
func (t *urlTokenizer) pairsIn(from, to int) {
	pair := 0
	for start := from; start <= to; {
		end := start + strings.IndexByte(t.masked[start:to]+"&", '&')
		if end > start {
			t.pair(start, end, pair)
			pair++
		}
		if end < to {
			t.fill(end, end+1, PartQueryMark)
		}
		start = end + 1
	}
}

func (t *urlTokenizer) pair(from, to, n int) {
	eq := strings.IndexByte(t.masked[from:to], '=')
	keyTo := to
	if eq >= 0 {
		keyTo = from + eq
		t.fill(keyTo, keyTo+1, PartEquals)
		t.fill(keyTo+1, to, PartValue)
	}
	t.fill(from, keyTo, PartKey)
	for i := from; i < to; i++ {
		if t.parts[i] != PartEquals { // "=" has its own colour, not the pair's
			t.pairs[i] = n
		}
	}
}

// overlayVariables marks the variables, unknown ones apart.
func (t *urlTokenizer) overlayVariables(known func(string) bool) {
	for n, v := range t.vars {
		part := PartVariable
		if known != nil && !known(t.src[v[0]+2:v[1]-2]) {
			part = PartUnknownVariable
		}
		for i := v[0]; i < v[1]; i++ {
			t.parts[i], t.ids[i] = part, n+1
		}
	}
}

// spans joins neighbouring bytes of the same part, pair and variable.
func (t *urlTokenizer) spans() []URLSpan {
	var out []URLSpan
	start := 0
	for i := 1; i <= len(t.src); i++ {
		if i < len(t.src) && t.parts[i] == t.parts[start] && t.pairs[i] == t.pairs[start] && t.ids[i] == t.ids[start] {
			continue
		}
		out = append(out, URLSpan{Text: t.src[start:i], Part: t.parts[start], Pair: t.pairs[start]})
		start = i
	}
	return out
}

// runtimeNamespaces are the names only a run can know: never reported as
// unknown while editing.
func runtimeNamespaces() []string {
	return []string{"vars", "with", "inputs", "nodes", "response", "request", "status"}
}

// NewNameChecker returns the "is this variable defined" test for
// TokenizeURL: env.x and bare names against the environment's variables,
// secret.x against its secrets, $name against the generators, and the
// run-time namespaces always.
func NewNameChecker(envNames, secretNames []string) func(expr string) bool {
	generators := map[string]bool{}
	for _, g := range template.Generators() {
		generators[g.Name] = true
	}
	return func(expr string) bool {
		expr = strings.TrimSpace(expr)
		if expr == "" {
			return false
		}
		if gen, ok := strings.CutPrefix(expr, "$"); ok {
			name, _, _ := strings.Cut(gen, "(")
			return generators[strings.TrimSpace(name)]
		}
		if !isPlainName(expr) {
			return true // an expression (a call, an operator, a literal): only a run can judge it
		}
		root, rest := splitRoot(expr)
		switch {
		case root == "env":
			return slices.Contains(envNames, firstSegment(rest))
		case root == "secret":
			return slices.Contains(secretNames, firstSegment(rest))
		case slices.Contains(runtimeNamespaces(), root):
			return true
		}
		return slices.Contains(envNames, root)
	}
}

// splitRoot cuts the leading identifier off an expression.
func splitRoot(expr string) (root, rest string) {
	end := 0
	for end < len(expr) && (isLetter(expr[end]) || expr[end] == '_' || (end > 0 && expr[end] >= '0' && expr[end] <= '9')) {
		end++
	}
	return expr[:end], strings.TrimPrefix(expr[end:], ".")
}

// firstSegment is the name before the next dot, bracket or space.
func firstSegment(s string) string {
	end := strings.IndexAny(s, ".[ )?")
	if end < 0 {
		return s
	}
	return s[:end]
}

// isPlainName reports whether expr is just dotted names (env.base_url):
// the only form this check can judge.
func isPlainName(expr string) bool {
	if expr == "" || (!isLetter(expr[0]) && expr[0] != '_') {
		return false
	}
	for i := 0; i < len(expr); i++ {
		c := expr[i]
		if !isLetter(c) && c != '_' && c != '.' && c != '-' && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}
