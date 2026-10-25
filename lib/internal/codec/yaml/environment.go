// SPDX-License-Identifier: Apache-2.0

package yamlcodec

import (
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
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
		if _, isMap := e.value.(*ast.MappingNode); !isMap {
			out = append(out, model.EnvVar{Name: e.key, Value: m.d.scalarString(e.value, e.path, e.key, true)})
			continue
		}
		sm, _ := m.d.mapping(e.value, e.path, e.key)
		out = append(out, model.EnvVar{Name: e.key, Secret: sm.secretFlag()})
	}
	return out
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
	var vars yaml.MapSlice
	for _, v := range env.Vars {
		var value any = v.Value
		if v.Secret {
			value = yaml.MapSlice{{Key: "secret", Value: true}}
		}
		vars = append(vars, yaml.MapItem{Key: v.Name, Value: value})
	}
	o.put("vars", vars)
	return marshal(o.items)
}
