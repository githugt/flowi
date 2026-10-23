// SPDX-License-Identifier: Apache-2.0

package yamlcodec

import (
	"slices"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

// DecodeBlueprint reads a *.blueprint.yaml file: a flow body plus declared
// parameters, where a request or subflow reference may be {param: name}.
// path is relative to blueprints/ without the suffix.
func (c *Codec) DecodeBlueprint(file, path string, data []byte) (model.Blueprint, diag.List) {
	bp := model.Blueprint{Path: path, File: file}
	d, root, ok := c.parse(file, data)
	if !ok {
		return bp, d.diags
	}
	d.blueprint = true
	m, ok := d.mapping(root, "$", "blueprint")
	if !ok {
		return bp, d.diags
	}
	bp.Meta = m.meta()
	bp.Phase = m.phase()
	bp.Params = m.params()
	m.flowBody(&bp.Body)
	m.closeStrict()
	return bp, d.diags
}

// phase reads `phase:`, which is pre or post when given.
func (m *mapping) phase() string {
	e, ok := m.take("phase")
	if !ok {
		return ""
	}
	phase := m.d.scalarString(e.value, e.path, "phase", false)
	if phase != "pre" && phase != "post" {
		m.d.typeError(e.path, e.value, "phase", "pre or post")
	}
	return phase
}

func (m *mapping) params() []model.Param {
	pm, ok := m.child("params")
	if !ok {
		return nil
	}
	out := make([]model.Param, 0, len(pm.entries))
	for _, e := range pm.entries {
		dm, ok := m.d.mapping(e.value, e.path, e.key)
		if !ok {
			continue
		}
		p := model.Param{Name: e.key, Description: dm.str("description")}
		p.Type = dm.paramType(e)
		p.Default = dm.lenient("default")
		dm.closeStrict()
		out = append(out, p)
	}
	return out
}

// paramType reads `type:` and reports a missing or unknown type.
func (m *mapping) paramType(param entry) model.ParamType {
	e, ok := m.take("type")
	if !ok {
		m.d.add(diag.CodeBlueprintParamType, param.path, param.keyNode, map[string]string{"name": param.key, "detail": "no type (want " + paramTypeList() + ")"})
		return ""
	}
	t := model.ParamType(m.d.scalarString(e.value, e.path, "type", false))
	if !slices.Contains(model.ParamTypes(), t) {
		m.d.add(diag.CodeBlueprintParamType, e.path, e.value, map[string]string{"name": param.key, "detail": `unknown type "` + string(t) + `" (want ` + paramTypeList() + ")"})
	}
	return t
}

func paramTypeList() string {
	types := model.ParamTypes()
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = string(t)
	}
	return strings.Join(names, ", ")
}

// lenient reads a scalar as its literal text (a default of `true` or `3`).
func (m *mapping) lenient(key string) string {
	e, ok := m.take(key)
	if !ok {
		return ""
	}
	return m.d.scalarString(e.value, e.path, key, true)
}

// EncodeBlueprint writes a blueprint in canonical form.
func (c *Codec) EncodeBlueprint(bp model.Blueprint) ([]byte, error) {
	var o omap
	putMeta(&o, bp.Meta)
	o.put("phase", bp.Phase)
	o.put("params", encodeParams(bp.Params))
	putFlowBody(&o, bp.Body)
	return marshal(o.items)
}

func encodeParams(params []model.Param) yaml.MapSlice {
	var out yaml.MapSlice
	for _, p := range params {
		var o omap
		o.put("type", string(p.Type))
		o.put("description", p.Description)
		o.put("default", p.Default)
		out = append(out, yaml.MapItem{Key: p.Name, Value: o.items})
	}
	return out
}
