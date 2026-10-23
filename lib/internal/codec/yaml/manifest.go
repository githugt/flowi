// SPDX-License-Identifier: Apache-2.0

package yamlcodec

import (
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/settings"
)

// DecodeManifest reads a collection's flowi.yaml.
func (c *Codec) DecodeManifest(file string, data []byte) (model.Manifest, diag.List) {
	d, root, ok := c.parse(file, data)
	if !ok {
		return model.Manifest{File: file}, d.diags
	}
	m, ok := d.mapping(root, "$", "flowi.yaml")
	if !ok {
		return model.Manifest{File: file}, d.diags
	}
	man := model.Manifest{Format: int(m.integer("format")), Meta: m.meta(), File: file}
	man.DependsOn = m.dependencies()
	man.Settings = c.settingsChild(d, m, settings.LayerCollection)
	man.Hooks = m.hooks()
	m.closeStrict()
	return man, d.diags
}

func (m *mapping) dependencies() []model.Dependency {
	e, ok := m.take("depends_on")
	if !ok {
		return nil
	}
	items, ok := m.d.sequence(e.value, e.path, "depends_on")
	if !ok {
		return nil
	}
	out := make([]model.Dependency, 0, len(items))
	for i, item := range items {
		dm, ok := m.d.mapping(item, indexPath(e.path, i), "depends_on")
		if !ok {
			continue
		}
		out = append(out, model.Dependency{ID: dm.id(), Name: dm.str("name"), Repo: dm.str("repo")})
		dm.closeStrict()
	}
	return out
}

// EncodeManifest writes flowi.yaml in canonical form.
func (c *Codec) EncodeManifest(man model.Manifest) ([]byte, error) {
	var o omap
	o.putAlways("format", int64(man.Format))
	putMeta(&o, man.Meta)
	o.put("depends_on", encodeDependencies(man.DependsOn))
	o.put("settings", c.encodeSettings(man.Settings))
	o.put("hooks", encodeHooks(man.Hooks))
	return marshal(o.items)
}

func putMeta(o *omap, m model.Meta) {
	o.put("id", string(m.ID))
	o.put("name", m.Name)
	o.put("description", m.Description)
}

func encodeDependencies(deps []model.Dependency) []any {
	out := make([]any, 0, len(deps))
	for _, dep := range deps {
		var o omap
		o.put("id", string(dep.ID))
		o.put("name", dep.Name)
		o.put("repo", dep.Repo)
		out = append(out, o.items)
	}
	return out
}
