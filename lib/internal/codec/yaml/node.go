// SPDX-License-Identifier: Apache-2.0

package yamlcodec

import (
	"github.com/goccy/go-yaml/ast"

	"github.com/githugt/flowi-lib/model"
)

// nodes reads a list of flow nodes.
func (m *mapping) nodes(key string) []model.Node {
	e, ok := m.take(key)
	if !ok {
		return nil
	}
	return m.d.nodeList(e.value, e.path, key)
}

func (d *decoder) nodeList(n ast.Node, path, key string) []model.Node {
	items, ok := d.sequence(n, path, key)
	if !ok {
		return nil
	}
	out := make([]model.Node, 0, len(items))
	for i, item := range items {
		nm, ok := d.mapping(item, indexPath(path, i), key)
		if !ok {
			continue
		}
		out = append(out, nm.flowNode())
	}
	return out
}

// node reads one node. Common fields and the type-specific fields that
// validation needs are typed; every other key is kept in Params, in order.
func (m *mapping) flowNode() model.Node {
	n := model.Node{
		ID:          m.str("id"),
		Type:        model.NodeType(m.str("type")),
		Name:        m.str("name"),
		Description: m.str("description"),
		Needs:       m.strings("needs"),
		When:        m.str("when"),
		Concurrent:  m.boolean("concurrent"),
		OnError:     m.str("on_error"),
		Timeout:     m.duration("timeout"),
		Attach:      m.str("attach"),
		AttachAt:    m.str("attach_at"),
	}
	switch n.Type {
	case "request":
		n.Request = m.requestCall()
	case "subflow":
		n.Subflow = m.subflowCall()
	case "batch", "run_till":
		n.Body = m.nodes("body")
	case "parallel":
		n.Branches = m.branches()
	case "condition":
		n.Then, n.Else, n.Cases = m.nodes("then"), m.nodes("else"), m.cases()
	}
	n.Params = m.d.rawFields(&mapping{d: m.d, entries: m.rest()})
	return n
}

func (m *mapping) requestCall() *model.RequestCall {
	call := &model.RequestCall{
		Version: m.versionLabel("version"),
		Hooks:   m.hookControl(),
		With:    m.keyValues("with"),
		Headers: m.keyValues("headers"),
	}
	call.Request, _ = m.ref("request")
	return call
}

func (m *mapping) subflowCall() *model.SubflowCall {
	call := &model.SubflowCall{With: m.keyValues("with")}
	call.Flow, _ = m.ref("flow")
	return call
}

// hookControl reads `hooks: all|none` or `hooks: {skip: [pre, post]}`.
func (m *mapping) hookControl() model.HookControl {
	e, ok := m.take("hooks")
	if !ok {
		return model.HookControl{}
	}
	if _, isMap := e.value.(*ast.MappingNode); !isMap {
		return model.HookControl{Mode: m.d.scalarString(e.value, e.path, "hooks", false)}
	}
	hm, _ := m.d.mapping(e.value, e.path, "hooks")
	hc := model.HookControl{Mode: hm.str("mode"), Skip: hm.strings("skip")}
	hm.closeStrict()
	return hc
}

func (m *mapping) branches() []model.Branch {
	e, ok := m.take("branches")
	if !ok {
		return nil
	}
	items, ok := m.d.sequence(e.value, e.path, "branches")
	if !ok {
		return nil
	}
	out := make([]model.Branch, 0, len(items))
	for i, item := range items {
		bm, ok := m.d.mapping(item, indexPath(e.path, i), "branches")
		if !ok {
			continue
		}
		out = append(out, model.Branch{ID: bm.str("id"), Nodes: bm.nodes("nodes")})
		bm.closeStrict()
	}
	return out
}

func (m *mapping) cases() []model.Case {
	e, ok := m.take("cases")
	if !ok {
		return nil
	}
	items, ok := m.d.sequence(e.value, e.path, "cases")
	if !ok {
		return nil
	}
	out := make([]model.Case, 0, len(items))
	for i, item := range items {
		cm, ok := m.d.mapping(item, indexPath(e.path, i), "cases")
		if !ok {
			continue
		}
		out = append(out, model.Case{When: cm.str("when"), Nodes: cm.nodes("nodes")})
		cm.closeStrict()
	}
	return out
}

func encodeNodes(nodes []model.Node) []any {
	out := make([]any, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, encodeNode(n))
	}
	return out
}

func encodeNode(n model.Node) any {
	var o omap
	o.put("id", n.ID)
	o.put("type", string(n.Type))
	o.put("name", n.Name)
	o.put("description", n.Description)
	o.put("needs", stringsToYAML(n.Needs))
	o.put("when", n.When)
	o.put("concurrent", n.Concurrent)
	o.put("on_error", n.OnError)
	o.put("attach", n.Attach)
	o.put("attach_at", n.AttachAt)
	if n.Timeout > 0 {
		o.put("timeout", formatDuration(n.Timeout))
	}
	encodeRequestCall(&o, n.Request)
	encodeSubflowCall(&o, n.Subflow)
	// Type-specific options come before nested node lists so a container reads
	// as "configuration, then body".
	o.items = append(o.items, fieldsToYAML(n.Params)...)
	o.put("body", encodeNodes(n.Body))
	o.put("branches", encodeBranches(n.Branches))
	o.put("then", encodeNodes(n.Then))
	o.put("else", encodeNodes(n.Else))
	o.put("cases", encodeCases(n.Cases))
	return o.items
}

func encodeRequestCall(o *omap, c *model.RequestCall) {
	if c == nil {
		return
	}
	o.put("request", encodeRef(c.Request))
	o.put("version", c.Version)
	o.put("hooks", encodeHookControl(c.Hooks))
	o.put("with", keyValues(c.With))
	o.put("headers", keyValues(c.Headers))
}

func encodeSubflowCall(o *omap, c *model.SubflowCall) {
	if c == nil {
		return
	}
	o.put("flow", encodeRef(c.Flow))
	o.put("with", keyValues(c.With))
}

func encodeHookControl(h model.HookControl) any {
	if len(h.Skip) == 0 {
		if h.Mode == "" {
			return nil
		}
		return h.Mode
	}
	var o omap
	o.put("mode", h.Mode)
	o.put("skip", stringsToYAML(h.Skip))
	return o.items
}

func encodeBranches(bs []model.Branch) []any {
	out := make([]any, 0, len(bs))
	for _, b := range bs {
		var o omap
		o.put("id", b.ID)
		o.put("nodes", encodeNodes(b.Nodes))
		out = append(out, o.items)
	}
	return out
}

func encodeCases(cs []model.Case) []any {
	out := make([]any, 0, len(cs))
	for _, c := range cs {
		var o omap
		o.put("when", c.When)
		o.put("nodes", encodeNodes(c.Nodes))
		out = append(out, o.items)
	}
	return out
}
