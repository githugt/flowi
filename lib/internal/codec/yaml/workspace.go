// SPDX-License-Identifier: Apache-2.0

package yamlcodec

import (
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/settings"
)

// DecodeWorkspace reads workspace.yaml.
func (c *Codec) DecodeWorkspace(file string, data []byte) (model.Workspace, diag.List) {
	ws := model.Workspace{File: file}
	d, root, ok := c.parse(file, data)
	if !ok {
		return ws, d.diags
	}
	m, ok := d.mapping(root, "$", "workspace.yaml")
	if !ok {
		return ws, d.diags
	}
	ws.Meta = m.meta()
	ws.Collections = m.mounts()
	ws.Schedules = m.schedules()
	ws.Settings = c.settingsChild(d, m, settings.LayerWorkspace)
	m.closeStrict()
	return ws, d.diags
}

func (m *mapping) mounts() []model.Mount {
	e, ok := m.take("collections")
	if !ok {
		return nil
	}
	items, ok := m.d.sequence(e.value, e.path, "collections")
	if !ok {
		return nil
	}
	out := make([]model.Mount, 0, len(items))
	for i, item := range items {
		mm, ok := m.d.mapping(item, indexPath(e.path, i), "collections")
		if !ok {
			continue
		}
		out = append(out, model.Mount{ID: mm.id(), Path: mm.str("path")})
		mm.closeStrict()
	}
	return out
}

func (m *mapping) schedules() []model.Schedule {
	e, ok := m.take("schedules")
	if !ok {
		return nil
	}
	items, ok := m.d.sequence(e.value, e.path, "schedules")
	if !ok {
		return nil
	}
	out := make([]model.Schedule, 0, len(items))
	for i, item := range items {
		sm, ok := m.d.mapping(item, indexPath(e.path, i), "schedules")
		if !ok {
			continue
		}
		out = append(out, model.Schedule{
			Name: sm.str("name"), Flow: sm.str("flow"), Collection: sm.str("collection"), Cron: sm.str("cron"),
			Env: sm.str("env"), Inputs: sm.keyValues("inputs"), TZ: sm.str("tz"), Disabled: sm.boolean("disabled"),
		})
		sm.closeStrict()
	}
	return out
}

func encodeSchedules(list []model.Schedule) []any {
	out := make([]any, 0, len(list))
	for _, s := range list {
		var so omap
		so.put("name", s.Name)
		so.put("flow", s.Flow)
		so.put("collection", s.Collection)
		so.put("cron", s.Cron)
		so.put("env", s.Env)
		so.put("inputs", keyValues(s.Inputs))
		so.put("tz", s.TZ)
		so.put("disabled", s.Disabled)
		out = append(out, so.items)
	}
	return out
}

// EncodeWorkspace writes workspace.yaml in canonical form.
func (c *Codec) EncodeWorkspace(ws model.Workspace) ([]byte, error) {
	var o omap
	putMeta(&o, ws.Meta)
	mounts := make([]any, 0, len(ws.Collections))
	for _, mt := range ws.Collections {
		var mo omap
		mo.put("id", string(mt.ID))
		mo.put("path", mt.Path)
		mounts = append(mounts, mo.items)
	}
	o.put("collections", mounts)
	o.put("schedules", encodeSchedules(ws.Schedules))
	o.put("settings", c.encodeSettings(ws.Settings))
	return marshal(o.items)
}
