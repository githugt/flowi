// SPDX-License-Identifier: Apache-2.0

package yamlcodec

import (
	"time"

	"github.com/goccy/go-yaml/ast"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/yamlpath"
)

// decoder walks a parsed YAML tree, collecting every problem it finds rather
// than stopping at the first, and records positions straight from tokens.
type decoder struct {
	file  string
	diags diag.List
	// blueprint is set while decoding a blueprint, where a reference may be
	// {param: name} instead of {id, path}.
	blueprint bool
}

func (d *decoder) add(code diag.Code, path string, n ast.Node, args map[string]string) {
	d.diags = append(d.diags, diag.New(code, diag.Location{File: d.file, Path: path, Pos: positionOf(n)}, args))
}

func positionOf(n ast.Node) diag.Position {
	if n == nil || n.GetToken() == nil || n.GetToken().Position == nil {
		return diag.Position{}
	}
	p := n.GetToken().Position
	return diag.Position{Line: p.Line, Column: p.Column}
}

func (d *decoder) typeError(path string, n ast.Node, key, expected string) {
	d.add(diag.CodeYAMLType, path, n, map[string]string{"key": key, "expected": expected, "got": describe(n)})
}

// unwrap removes anchors and tags, which carry no meaning for Flowi.
// Aliases are rejected: they would let one file silently share values.
func (d *decoder) unwrap(n ast.Node, path, key string) (ast.Node, bool) {
	for {
		switch v := n.(type) {
		case *ast.AnchorNode:
			n = v.Value
		case *ast.TagNode:
			n = v.Value
		case *ast.AliasNode:
			d.typeError(path, n, key, "a value (YAML aliases are not supported)")
			return nil, false
		default:
			return n, true
		}
	}
}

func describe(n ast.Node) string {
	switch n.(type) {
	case *ast.MappingNode:
		return "a mapping"
	case *ast.SequenceNode:
		return "a list"
	case *ast.NullNode, nil:
		return "nothing"
	case *ast.BoolNode:
		return "a boolean"
	case *ast.IntegerNode, *ast.FloatNode:
		return "a number"
	default:
		return "text"
	}
}

func isNull(n ast.Node) bool {
	_, null := n.(*ast.NullNode)
	return n == nil || null
}

// entry is one key/value pair of a mapping, in file order.
type entry struct {
	key     string
	keyNode ast.Node
	value   ast.Node
	path    string
}

// mapping is a decoded YAML mapping. Keys are taken from the raw token text
// so that "1.10:" stays "1.10" instead of becoming the float 1.1.
type mapping struct {
	d       *decoder
	path    string
	node    ast.Node
	entries []entry
	used    map[string]bool
}

func (d *decoder) mapping(n ast.Node, path, key string) (*mapping, bool) {
	n, ok := d.unwrap(n, path, key)
	if !ok {
		return nil, false
	}
	if isNull(n) {
		return &mapping{d: d, path: path, node: n, used: map[string]bool{}}, true
	}
	mn, isMap := n.(*ast.MappingNode)
	if !isMap {
		d.typeError(path, n, key, "a mapping")
		return nil, false
	}
	m := &mapping{d: d, path: path, node: n, used: map[string]bool{}}
	seen := map[string]bool{}
	for _, mv := range mn.Values {
		k := mv.Key.GetToken().Value
		if seen[k] {
			d.add(diag.CodeYAMLDuplicateKey, childPath(path, k), mv.Key, map[string]string{"key": k})
			continue
		}
		seen[k] = true
		m.entries = append(m.entries, entry{key: k, keyNode: mv.Key, value: mv.Value, path: childPath(path, k)})
	}
	return m, true
}

func childPath(parent, key string) string { return yamlpath.Key(parent, key) }

func indexPath(parent string, i int) string { return yamlpath.Index(parent, i) }

// take returns the entry for key and marks it consumed.
func (m *mapping) take(key string) (entry, bool) {
	for _, e := range m.entries {
		if e.key == key {
			m.used[key] = true
			return e, !isNull(e.value)
		}
	}
	return entry{}, false
}

// rest returns the entries not yet consumed, in file order.
func (m *mapping) rest() []entry {
	var out []entry
	for _, e := range m.entries {
		if !m.used[e.key] {
			out = append(out, e)
		}
	}
	return out
}

// closeStrict reports every entry nobody consumed as an unknown key.
func (m *mapping) closeStrict() {
	for _, e := range m.rest() {
		m.d.add(diag.CodeYAMLUnknownKey, e.path, e.keyNode, map[string]string{"key": e.key})
	}
}

func (m *mapping) str(key string) string {
	e, ok := m.take(key)
	if !ok {
		return ""
	}
	return m.d.scalarString(e.value, e.path, key, false)
}

// scalarString reads a scalar as text. Numbers and booleans are accepted
// as their literal text only when lenient (header and variable values),
// because users write `port: 8080` and mean the text "8080".
func (d *decoder) scalarString(n ast.Node, path, key string, lenient bool) string {
	n, ok := d.unwrap(n, path, key)
	if !ok {
		return ""
	}
	switch v := n.(type) {
	case *ast.StringNode:
		return v.Value
	case *ast.LiteralNode:
		return v.Value.Value
	case *ast.NullNode:
		return ""
	case *ast.IntegerNode, *ast.FloatNode, *ast.BoolNode:
		if lenient {
			return v.GetToken().Value
		}
	}
	d.typeError(path, n, key, "text")
	return ""
}

func (m *mapping) integer(key string) int64 {
	e, ok := m.take(key)
	if !ok {
		return 0
	}
	n, ok := m.d.unwrap(e.value, e.path, key)
	if !ok {
		return 0
	}
	if in, isInt := n.(*ast.IntegerNode); isInt {
		if v, ok := normaliseInt(in.Value).(int64); ok {
			return v
		}
	}
	m.d.typeError(e.path, n, key, "a whole number")
	return 0
}

func (m *mapping) boolean(key string) bool {
	e, ok := m.take(key)
	if !ok {
		return false
	}
	n, ok := m.d.unwrap(e.value, e.path, key)
	if !ok {
		return false
	}
	if b, isBool := n.(*ast.BoolNode); isBool {
		return b.Value
	}
	m.d.typeError(e.path, n, key, "true or false")
	return false
}

func (m *mapping) duration(key string) time.Duration {
	e, ok := m.take(key)
	if !ok {
		return 0
	}
	s := m.d.scalarString(e.value, e.path, key, false)
	if s == "" {
		return 0
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		m.d.typeError(e.path, e.value, key, "a duration such as 30s")
		return 0
	}
	return v
}

func (m *mapping) strings(key string) []string {
	e, ok := m.take(key)
	if !ok {
		return nil
	}
	items, ok := m.d.sequence(e.value, e.path, key)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for i, item := range items {
		out = append(out, m.d.scalarString(item, indexPath(e.path, i), key, false))
	}
	return out
}

func (d *decoder) sequence(n ast.Node, path, key string) ([]ast.Node, bool) {
	n, ok := d.unwrap(n, path, key)
	if !ok {
		return nil, false
	}
	if isNull(n) {
		return nil, true
	}
	seq, isSeq := n.(*ast.SequenceNode)
	if !isSeq {
		d.typeError(path, n, key, "a list")
		return nil, false
	}
	return seq.Values, true
}

// raw converts any YAML value into plain Go values: string, bool, int64,
// uint64, float64, nil, []any or an ordered []field.
func (d *decoder) raw(n ast.Node, path, key string) any {
	n, ok := d.unwrap(n, path, key)
	if !ok {
		return nil
	}
	switch v := n.(type) {
	case *ast.StringNode:
		return v.Value
	case *ast.LiteralNode:
		return v.Value.Value
	case *ast.IntegerNode:
		return normaliseInt(v.Value)
	case *ast.FloatNode:
		return v.Value
	case *ast.BoolNode:
		return v.Value
	case *ast.NullNode, nil:
		return nil
	case *ast.SequenceNode:
		out := make([]any, 0, len(v.Values))
		for i, item := range v.Values {
			out = append(out, d.raw(item, indexPath(path, i), key))
		}
		return out
	case *ast.MappingNode:
		m, _ := d.mapping(v, path, key)
		return d.rawFields(m)
	}
	d.typeError(path, n, key, "a value")
	return nil
}

// normaliseInt returns int64 for every integer that fits, since the parser
// yields uint64 for positive literals; larger values stay uint64.
func normaliseInt(v any) any {
	if u, ok := v.(uint64); ok && u <= 1<<63-1 {
		return int64(u)
	}
	return v
}

// startNode is the node a whole-mapping diagnostic points at: its first key.
func (m *mapping) startNode() ast.Node {
	if len(m.entries) > 0 {
		return m.entries[0].keyNode
	}
	return m.node
}
