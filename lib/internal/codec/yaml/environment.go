// SPDX-License-Identifier: Apache-2.0

package yamlcodec

import (
	"encoding/json"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// DecodeEnvironment reads an environments/*.yaml file. Each variable is a
// scalar value or {secret: true}; secret values never live in the file.
func (c *Codec) DecodeEnvironment(file string, data []byte) (model.Environment, diag.List) {
	env := model.Environment{File: file}
	d, root, ok := c.parse(file, data)
	if !ok {
		return env, d.diags
	}
	m, ok := d.mapping(root, "$", "environment")
	if !ok {
		return env, d.diags
	}
	env.Meta = m.meta()
	env.Colour = m.str("colour")
	env.Extends = m.str("extends")
	env.Vars = m.envVars()
	m.closeStrict()
	return env, d.diags
}

func (m *mapping) envVars() []model.EnvVar {
	vm, ok := m.child("vars")
	if !ok {
		return nil
	}
	out := make([]model.EnvVar, 0, len(vm.entries))
	for _, e := range vm.entries {
		out = append(out, m.envVar(e))
	}
	return out
}

// envVar reads one variable. A scalar keeps its YAML type: 5 is a number, true a boolean, "5" text. A
// mapping with a `secret` key is a secret; any other mapping, and a list, is an object held as JSON.
func (m *mapping) envVar(e entry) model.EnvVar {
	n, ok := m.d.unwrap(e.value, e.path, e.key)
	if !ok {
		return model.EnvVar{Name: e.key}
	}
	switch v := n.(type) {
	case *ast.IntegerNode, *ast.FloatNode:
		return model.EnvVar{Name: e.key, Value: v.GetToken().Value, Kind: model.VarNumber}
	case *ast.BoolNode:
		return model.EnvVar{Name: e.key, Value: strconv.FormatBool(v.Value), Kind: model.VarBoolean}
	case *ast.SequenceNode:
		return m.jsonVar(e, n)
	case *ast.MappingNode:
		sm, _ := m.d.mapping(e.value, e.path, e.key)
		if sm.has("secret") {
			return model.EnvVar{Name: e.key, Secret: sm.secretFlag()}
		}
		return m.jsonVar(e, n)
	}
	return model.EnvVar{Name: e.key, Value: m.d.scalarString(e.value, e.path, e.key, true)}
}

// jsonVar holds an object or a list as compact JSON text.
func (m *mapping) jsonVar(e entry, n ast.Node) model.EnvVar {
	var v any
	if err := yaml.NodeToValue(n, &v); err != nil {
		m.d.typeError(e.path, n, e.key, "a value that can be written as JSON")
		return model.EnvVar{Name: e.key}
	}
	data, err := json.Marshal(v)
	if err != nil {
		m.d.typeError(e.path, n, e.key, "a value that can be written as JSON")
		return model.EnvVar{Name: e.key}
	}
	return model.EnvVar{Name: e.key, Value: string(data), Kind: model.VarJSON}
}

// secretFlag reads {secret: true}. A value next to it is reported and
// dropped so it cannot be committed by accident through `flowi fmt`.
func (m *mapping) secretFlag() bool {
	secret := m.boolean("secret")
	if e, ok := m.take("value"); ok {
		m.d.typeError(e.path, e.keyNode, "value", "absent (secret values belong in the vault)")
	}
	m.closeStrict()
	return secret
}

// EncodeEnvironment writes an environment in canonical form.
func (c *Codec) EncodeEnvironment(env model.Environment) ([]byte, error) {
	var o omap
	putMeta(&o, env.Meta)
	o.put("colour", env.Colour)
	o.put("extends", env.Extends)
	var vars yaml.MapSlice
	for _, v := range env.Vars {
		vars = append(vars, yaml.MapItem{Key: v.Name, Value: envVarValue(v)})
	}
	o.put("vars", vars)
	return marshal(o.items)
}

// envVarValue is what a variable is written as. A value that does not fit its kind is written as text, so
// a typo is kept for the person to fix and `flowi validate` can say what is wrong.
func envVarValue(v model.EnvVar) any {
	if v.Secret {
		return yaml.MapSlice{{Key: "secret", Value: true}}
	}
	if v.Kind == model.VarString {
		return v.Value
	}
	if v.Kind == model.VarJSON {
		return jsonVarValue(v)
	}
	typed, err := v.Typed()
	if err != nil {
		return v.Value
	}
	return typed
}

// jsonVarValue decodes an object variable so that a whole number is written as 3, not 3.0.
func jsonVarValue(v model.EnvVar) any {
	dec := json.NewDecoder(strings.NewReader(v.Value))
	dec.UseNumber()
	var out any
	if err := dec.Decode(&out); err != nil {
		return v.Value
	}
	return wholeNumbers(out)
}

func wholeNumbers(v any) any {
	switch x := v.(type) {
	case json.Number:
		if n, err := x.Int64(); err == nil {
			return n
		}
		f, _ := x.Float64()
		return f
	case map[string]any:
		items := make(yaml.MapSlice, 0, len(x))
		for _, k := range slices.Sorted(maps.Keys(x)) {
			items = append(items, yaml.MapItem{Key: k, Value: wholeNumbers(x[k])})
		}
		return items
	case []any:
		for i := range x {
			x[i] = wholeNumbers(x[i])
		}
	}
	return v
}

// has says whether the mapping still holds an entry named key.
func (m *mapping) has(key string) bool {
	return slices.ContainsFunc(m.entries, func(e entry) bool { return e.key == key })
}
