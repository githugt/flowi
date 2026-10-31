// SPDX-License-Identifier: Apache-2.0

// Package xmlpath selects values from an XML document with the small part of XPath that reading an answer needs: absolute
// and "//" paths, element names (matched by local name, since a server's namespace prefix is its own choice), "*",
// "[n]", "[last()]", "[@attr='value']", "@attr" and "text()". It is not XPath: no axes, functions or predicates on child
// text. The WebDAV 207 answer is the case it is for.
package xmlpath

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// maxDepth refuses a document nested deeper than any real answer: the walk keeps the whole tree.
const maxDepth = 1000

type node struct {
	name     string
	attrs    map[string]string
	children []*node
	text     strings.Builder // the character data directly inside
}

// Select returns the text of each node path selects, in document order: an element's own and its descendants' text, an
// attribute's value, or the direct text for text(). It returns none when nothing matches; an error says the path or the
// document cannot be read.
func Select(doc, path string) ([]string, error) {
	steps, err := parsePath(path)
	if err != nil {
		return nil, err
	}
	root, err := parseDocument(doc)
	if err != nil {
		return nil, err
	}
	return evaluate(&node{children: []*node{root}}, steps), nil
}

func parseDocument(doc string) (*node, error) {
	dec := xml.NewDecoder(strings.NewReader(doc))
	dec.Strict = false
	var stack []*node
	var root *node
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("the body is not XML: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if len(stack) >= maxDepth {
				return nil, fmt.Errorf("the XML is nested more than %d deep", maxDepth)
			}
			n := &node{name: t.Name.Local, attrs: map[string]string{}}
			for _, a := range t.Attr {
				n.attrs[a.Name.Local] = a.Value
			}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, n)
			} else if root == nil {
				root = n
			}
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text.Write(t)
			}
		}
	}
	if root == nil {
		return nil, errors.New("the body is not XML: it has no element")
	}
	return root, nil
}

type step struct {
	deep      bool   // reached through "//"
	kind      string // element, attribute or text
	name      string // "*" for any element
	index     int    // 1-based, 0 for none
	last      bool
	attrName  string
	attrValue string
	hasAttr   bool
}

func parsePath(path string) ([]step, error) {
	if !strings.HasPrefix(path, "/") {
		return nil, fmt.Errorf("XML path %q must start with / (or //)", path)
	}
	var steps []step
	rest := path
	for rest != "" {
		deep := strings.HasPrefix(rest, "//")
		rest = strings.TrimPrefix(strings.TrimPrefix(rest, "/"), "/")
		text, after := cutStep(rest)
		if text == "" {
			return nil, fmt.Errorf("XML path %q has an empty step", path)
		}
		s, err := parseStep(text)
		if err != nil {
			return nil, err
		}
		s.deep = deep
		steps = append(steps, s)
		rest = after
	}
	for i, s := range steps {
		if s.kind != "element" && i != len(steps)-1 {
			return nil, fmt.Errorf("XML path %q: @attribute and text() must be last", path)
		}
	}
	return steps, nil
}

// cutStep splits the first step off rest: up to the next "/" that is not inside a predicate or a quote.
func cutStep(rest string) (step, after string) {
	depth, quote := 0, rune(0)
	for i, r := range rest {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
		case r == '[':
			depth++
		case r == ']':
			depth--
		case r == '/' && depth == 0:
			return rest[:i], rest[i:]
		}
	}
	return rest, ""
}

func parseStep(text string) (step, error) {
	name, predicate, hasPredicate := strings.Cut(text, "[")
	s := step{kind: "element", name: localName(name)}
	switch {
	case name == "text()":
		s.kind, s.name = "text", ""
	case strings.HasPrefix(name, "@"):
		s.kind, s.name = "attribute", localName(strings.TrimPrefix(name, "@"))
	}
	if s.name == "" && s.kind != "text" {
		return step{}, fmt.Errorf("XML path step %q has no name", text)
	}
	if !hasPredicate {
		return s, nil
	}
	if s.kind != "element" {
		return step{}, fmt.Errorf("XML path step %q: only an element takes a predicate", text)
	}
	if !strings.HasSuffix(predicate, "]") {
		return step{}, fmt.Errorf("XML path step %q: a predicate is missing its closing ]", text)
	}
	return parsePredicate(s, strings.TrimSuffix(predicate, "]"), text)
}

func parsePredicate(s step, p, text string) (step, error) {
	switch {
	case p == "last()":
		s.last = true
	case strings.HasPrefix(p, "@"):
		name, value, ok := strings.Cut(p[1:], "=")
		value = strings.TrimSpace(value)
		if !ok || len(value) < 2 || (value[0] != '\'' && value[0] != '"') {
			return step{}, fmt.Errorf("XML path step %q: write the predicate as [@name='value']", text)
		}
		if value[len(value)-1] != value[0] {
			return step{}, fmt.Errorf("XML path step %q: the quote in the predicate is not closed", text)
		}
		s.hasAttr, s.attrName, s.attrValue = true, localName(strings.TrimSpace(name)), value[1:len(value)-1]
	default:
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return step{}, fmt.Errorf("XML path step %q: the predicate must be a number, last() or @name='value'", text)
		}
		if n < 1 {
			return step{}, fmt.Errorf("XML path step %q: positions count from 1", text)
		}
		s.index = n
	}
	return s, nil
}

// localName drops a namespace prefix: D:href is href.
func localName(name string) string {
	if i := strings.LastIndex(name, ":"); i >= 0 {
		return name[i+1:]
	}
	return name
}

func evaluate(docRoot *node, steps []step) []string {
	context := []*node{docRoot}
	for i, s := range steps {
		if s.kind != "element" {
			return terminal(context, s)
		}
		var next []*node
		seen := map[*node]bool{}
		for _, c := range contextsFor(context, s.deep) {
			for _, n := range pick(c, s) {
				if !seen[n] {
					seen[n] = true
					next = append(next, n)
				}
			}
		}
		context = next
		if i == len(steps)-1 {
			out := make([]string, len(context))
			for j, n := range context {
				out[j] = strings.TrimSpace(textOf(n))
			}
			return out
		}
	}
	return nil
}

// contextsFor are the nodes whose children a step looks at: the context nodes, and with "//" everything below them too.
func contextsFor(context []*node, deep bool) []*node {
	if !deep {
		return context
	}
	var out []*node
	seen := map[*node]bool{}
	var walk func(n *node)
	walk = func(n *node) {
		if seen[n] {
			return
		}
		seen[n] = true
		out = append(out, n)
		for _, c := range n.children {
			walk(c)
		}
	}
	for _, c := range context {
		walk(c)
	}
	return out
}

// pick are the children of c a step selects, after its predicate.
func pick(c *node, s step) []*node {
	var matched []*node
	for _, child := range c.children {
		if (s.name == "*" || child.name == s.name) && (!s.hasAttr || child.attrs[s.attrName] == s.attrValue) {
			matched = append(matched, child)
		}
	}
	switch {
	case s.last && len(matched) > 0:
		return matched[len(matched)-1:]
	case s.index > 0 && s.index <= len(matched):
		return matched[s.index-1 : s.index]
	case s.index > 0:
		return nil
	}
	return matched
}

// terminal reads an attribute or the direct text of each context node (and, with "//", of those below).
func terminal(context []*node, s step) []string {
	var out []string
	for _, c := range contextsFor(context, s.deep) {
		switch s.kind {
		case "attribute":
			if v, ok := c.attrs[s.name]; ok && c.name != "" {
				out = append(out, v)
			}
		case "text":
			if c.name != "" {
				out = append(out, strings.TrimSpace(c.text.String()))
			}
		}
	}
	return out
}

// textOf is the text of n and everything below it, in document order.
func textOf(n *node) string {
	var b strings.Builder
	var walk func(*node)
	walk = func(x *node) {
		b.WriteString(x.text.String())
		for _, c := range x.children {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}
