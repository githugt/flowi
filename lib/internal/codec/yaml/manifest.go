// SPDX-License-Identifier: Apache-2.0

package yamlcodec

import (
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/settings"
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
	man := model.Manifest{Format: int(m.integer("format")), Meta: m.meta(), MinVersion: m.str("min_version"), File: file}
	man.DependsOn = m.dependencies()
	man.Variables = m.keyValues("variables")
	man.Statuses = m.statuses()
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
	o.put("min_version", man.MinVersion)
	o.put("depends_on", encodeDependencies(man.DependsOn))
	o.put("variables", keyValues(man.Variables))
	o.put("statuses", encodeStatuses(man.Statuses))
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

// statuses reads the statuses a collection adds: a list of {name, color, icon}. Whether a name or colour is usable is
// the validator's business (FLW-STATUS-002), so the file still loads and the problem is reported with its place.
func (m *mapping) statuses() []model.Status {
	e, ok := m.take("statuses")
	if !ok {
		return nil
	}
	items, ok := m.d.sequence(e.value, e.path, "statuses")
	if !ok {
		return nil
	}
	out := make([]model.Status, 0, len(items))
	for i, item := range items {
		sm, ok := m.d.mapping(item, indexPath(e.path, i), "statuses")
		if !ok {
			continue
		}
		out = append(out, model.Status{Name: sm.str("name"), Color: sm.str("color"), Icon: sm.str("icon")})
		sm.closeStrict()
	}
	return out
}

func encodeStatuses(list []model.Status) []any {
	out := make([]any, 0, len(list))
	for _, s := range list {
		var o omap
		o.put("name", s.Name)
		o.put("color", s.Color)
		o.put("icon", s.Icon)
		out = append(out, o.items)
	}
	return out
}
