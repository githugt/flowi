// SPDX-License-Identifier: Apache-2.0

package mermaid

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/githugt/flowi-lib/diag"
)

// graph is a parsed flowchart: nodes in order of first appearance, edges
// in order, and what does not import (subgraph titles, edge labels).
type graph struct {
	nodes      []*gnode
	byID       map[string]*gnode
	edges      []edge
	subgraphs  []string
	edgeLabels []string
	// open is the stack of subgraph ids being read; members lists the nodes (and
	// subgraphs) written inside each, so an arrow to a subgraph can be resolved.
	open    []string
	members map[string][]string
}

type gnode struct {
	id, label string
	line, col int // where the node first appears
	order     int
}

type edge struct{ from, to string }

type syntaxError struct {
	pos    diag.Position
	detail string
}

func (e *syntaxError) Error() string { return e.detail }

var (
	header  = regexp.MustCompile(`^(flowchart|graph)(\s+(TB|TD|BT|RL|LR))?\s*;?$`)
	nodeID  = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_-]*`)
	ignored = regexp.MustCompile(`^(style|classDef|class|linkStyle|click|direction|accTitle|accDescr)\b`)
	// link matches an arrow and an optional |label|: -->, ---, -.->,
	// ==>, ===, --o, --x, <-->, and longer dashes.
	link      = regexp.MustCompile(`^(<?)(-{2,}|={2,}|-\.+-)(>|o|x)?(?:\|([^|]*)\|)?`)
	textLink  = regexp.MustCompile(`^--\s+([^-|>][^>]*?)\s+-->`)
	subgraphR = regexp.MustCompile(`^subgraph\s+(.*)$`)
)

// shapes are node shape delimiters, longest opener first.
func shapes() [][2]string {
	return [][2]string{
		{"([", "])"}, {"[[", "]]"}, {"[(", ")]"}, {"((", "))"}, {"{{", "}}"},
		{"[/", "/]"}, {"[\\", "\\]"}, {"[/", "\\]"}, {"[\\", "/]"},
		{"[", "]"}, {"(", ")"}, {"{", "}"}, {">", "]"},
	}
}

// parse reads a flowchart. Statements end at a newline or ";".
func parse(src []byte) (*graph, error) {
	g := &graph{byID: map[string]*gnode{}, members: map[string][]string{}}
	lines := strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")
	seenHeader := false
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "%%") {
			continue
		}
		if !seenHeader {
			if !header.MatchString(t) {
				return nil, &syntaxError{pos: diag.Position{Line: i + 1, Column: 1}, detail: "a Mermaid sketch starts with flowchart or graph"}
			}
			seenHeader = true
			continue
		}
		if err := g.line(line, i+1); err != nil {
			return nil, err
		}
	}
	if !seenHeader {
		return nil, &syntaxError{pos: diag.Position{Line: 1, Column: 1}, detail: "a Mermaid sketch starts with flowchart or graph"}
	}
	g.resolveSubgraphs()
	return g, nil
}

// line reads one line's statements.
func (g *graph) line(line string, n int) error {
	for _, stmt := range splitStatements(line) {
		t := strings.TrimSpace(stmt.text)
		switch {
		case t == "end":
			g.closeSubgraph()
		case t == "" || ignored.MatchString(t):
		case subgraphR.MatchString(t):
			g.openSubgraph(strings.TrimSpace(subgraphR.FindStringSubmatch(t)[1]))
		default:
			col := stmt.col + utf8.RuneCountInString(stmt.text) - utf8.RuneCountInString(strings.TrimLeft(stmt.text, " \t"))
			if err := g.chain(t, n, col); err != nil {
				return err
			}
		}
	}
	return nil
}

type statement struct {
	text string
	col  int // rune column where it starts
}

// splitStatements splits at ";" outside brackets and quotes.
func splitStatements(line string) []statement {
	var out []statement
	depth, quoted, start, col, startCol := 0, false, 0, 1, 1
	for i, r := range line {
		switch {
		case r == '"':
			quoted = !quoted
		case quoted:
		case strings.ContainsRune("[({", r):
			depth++
		case strings.ContainsRune("])}", r) && depth > 0:
			depth--
		case r == ';' && depth == 0:
			out = append(out, statement{text: line[start:i], col: startCol})
			start, startCol = i+1, col+1
		}
		col++
	}
	return append(out, statement{text: line[start:], col: startCol})
}

// chain reads node groups joined by links: a & b --> c -->|x| d.
func (g *graph) chain(s string, line, col int) error {
	c := &cursor{s: s, line: line, col: col}
	prev, err := g.group(c)
	if err != nil {
		return err
	}
	for c.skipSpace(); !c.done(); c.skipSpace() {
		if err := g.link(c); err != nil {
			return err
		}
		c.skipSpace()
		next, err := g.group(c)
		if err != nil {
			return err
		}
		for _, a := range prev {
			for _, b := range next {
				g.edges = append(g.edges, edge{from: a, to: b})
			}
		}
		prev = next
	}
	return nil
}

func (g *graph) link(c *cursor) error {
	rest := c.rest()
	if m := textLink.FindStringSubmatch(rest); m != nil {
		g.edgeLabels = append(g.edgeLabels, strings.TrimSpace(m[1]))
		c.advance(len(m[0]))
		return nil
	}
	m := link.FindStringSubmatch(rest)
	if m == nil {
		return c.errorf("expected an arrow such as -->")
	}
	if m[4] != "" {
		g.edgeLabels = append(g.edgeLabels, strings.TrimSpace(m[4]))
	}
	c.advance(len(m[0]))
	return nil
}

// group reads node (& node)*.
func (g *graph) group(c *cursor) ([]string, error) {
	var ids []string
	for {
		c.skipSpace()
		id, err := g.node(c)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
		c.skipSpace()
		if !strings.HasPrefix(c.rest(), "&") {
			return ids, nil
		}
		c.advance(1)
	}
}

// node reads an id and an optional shape with a label.
func (g *graph) node(c *cursor) (string, error) {
	line, col := c.line, c.position()
	id := trimArrow(nodeID.FindString(c.rest()), c.rest())
	if id == "" {
		return "", c.errorf("expected a node id")
	}
	c.advance(len(id))
	label, shaped, err := shapeLabel(c)
	if err != nil {
		return "", err
	}
	n, ok := g.byID[id]
	if !ok {
		n = &gnode{id: id, label: id, line: line, col: col, order: len(g.nodes)}
		g.byID[id] = n
		g.nodes = append(g.nodes, n)
	}
	if shaped && n.label == n.id {
		n.label, n.line, n.col = label, line, col
	}
	g.addToOpen(id)
	return id, nil
}

// trimArrow cuts an id where an arrow starts: ids may contain "-", so
// "A-->B" first matches "A--" and "B-.->C" matches "B-".
func trimArrow(id, rest string) string {
	if i := strings.Index(id, "--"); i >= 0 {
		id = id[:i]
	}
	if strings.HasSuffix(id, "-") && len(rest) > len(id) && strings.ContainsRune(".>", rune(rest[len(id)])) {
		id = strings.TrimSuffix(id, "-")
	}
	return id
}

// shapeLabel reads a shape's text if one starts at the cursor.
func shapeLabel(c *cursor) (string, bool, error) {
	rest := c.rest()
	for _, s := range shapes() {
		if !strings.HasPrefix(rest, s[0]) {
			continue
		}
		openAt := c.position()
		body := rest[len(s[0]):]
		var text string
		if strings.HasPrefix(body, `"`) {
			end := strings.Index(body[1:], `"`)
			if end < 0 || !strings.HasPrefix(body[end+2:], s[1]) {
				continue
			}
			text = body[1 : end+1]
			c.advance(len(s[0]) + end + 2 + len(s[1]))
		} else {
			end := strings.Index(body, s[1])
			if end < 0 {
				if s[0] == "[" || s[0] == "(" || s[0] == "{" {
					return "", false, &syntaxError{pos: diag.Position{Line: c.line, Column: openAt}, detail: fmt.Sprintf("the shape %s… is never closed with %s", s[0], s[1])}
				}
				continue
			}
			text = body[:end]
			c.advance(len(s[0]) + end + len(s[1]))
		}
		return decodeLabel(text), true, nil
	}
	return "", false, nil
}

// decodeLabel undoes Mermaid's entities and line breaks.
func decodeLabel(s string) string {
	s = strings.NewReplacer("#quot;", `"`, "<br/>", "\n", "<br />", "\n", "<br>", "\n", "#35;", "#", "#59;", ";").Replace(s)
	return strings.TrimSpace(s)
}

// cursor walks a statement, tracking the rune column for messages.
type cursor struct {
	s         string
	i         int
	line, col int // col of s[0]
}

func (c *cursor) rest() string { return c.s[c.i:] }
func (c *cursor) done() bool   { return c.i >= len(c.s) }
func (c *cursor) advance(n int) {
	c.i = min(c.i+n, len(c.s))
}
func (c *cursor) skipSpace() {
	for c.i < len(c.s) && (c.s[c.i] == ' ' || c.s[c.i] == '\t') {
		c.i++
	}
}
func (c *cursor) position() int { return c.col + utf8.RuneCountInString(c.s[:c.i]) }
func (c *cursor) errorf(format string, args ...any) error {
	return &syntaxError{pos: diag.Position{Line: c.line, Column: c.position()}, detail: fmt.Sprintf(format, args...)}
}
