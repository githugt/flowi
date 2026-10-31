// SPDX-License-Identifier: Apache-2.0

package yamlcodec

import (
	"maps"
	"slices"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

// DecodeLayout reads a *.layout.yaml file. Problems are diagnostics; what
// could be read is returned (a layout is only a convenience).
func (c *Codec) DecodeLayout(file string, data []byte) (model.Layout, diag.List) {
	l := model.Layout{File: file, Nodes: map[string]model.Point{}}
	d, root, ok := c.parse(file, data)
	if !ok {
		return l, d.diags
	}
	m, ok := d.mapping(root, "$", "layout")
	if !ok {
		return l, d.diags
	}
	l.Flow = model.ID(m.str("flow"))
	if nodes, ok := m.child("nodes"); ok {
		for _, e := range nodes.entries {
			pm, ok := d.mapping(e.value, e.path, e.key)
			if !ok {
				continue
			}
			nodes.used[e.key] = true
			l.Nodes[e.key] = model.Point{X: pm.integer("x"), Y: pm.integer("y")}
			pm.closeStrict()
		}
	}
	l.Collapsed = m.stringList("collapsed")
	m.closeStrict()
	return l, d.diags
}

// stringList reads a sequence of scalars.
func (m *mapping) stringList(key string) []string {
	e, ok := m.take(key)
	if !ok {
		return nil
	}
	n, ok := m.d.unwrap(e.value, e.path, key)
	if !ok {
		return nil
	}
	seq, isSeq := n.(*ast.SequenceNode)
	if !isSeq {
		m.d.typeError(e.path, n, key, "a list")
		return nil
	}
	var out []string
	for i, item := range seq.Values {
		if s := m.d.scalarString(item, indexPath(e.path, i), key, true); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// EncodeLayout writes a layout in canonical form: positions sorted by
// node id.
func (c *Codec) EncodeLayout(l model.Layout) ([]byte, error) {
	var o omap
	o.put("flow", string(l.Flow))
	var nodes yaml.MapSlice
	for _, id := range slices.Sorted(maps.Keys(l.Nodes)) {
		p := l.Nodes[id]
		nodes = append(nodes, yaml.MapItem{Key: id, Value: yaml.MapSlice{{Key: "x", Value: p.X}, {Key: "y", Value: p.Y}}})
	}
	o.put("nodes", nodes)
	o.put("collapsed", slices.Sorted(slices.Values(l.Collapsed)))
	return marshal(o.items)
}
