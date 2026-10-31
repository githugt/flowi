// SPDX-License-Identifier: Apache-2.0

package yamlcodec

import (
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/ids"
	"github.com/githugt/flowi/lib/model"
)

func (d *decoder) rawFields(m *mapping) []model.Field {
	if m == nil {
		return nil
	}
	var out []model.Field
	for _, e := range m.entries {
		out = append(out, model.Field{Key: e.key, Value: d.raw(e.value, e.path, e.key)})
	}
	return out
}

// keyValues reads a mapping of names to scalar values in file order.
func (m *mapping) keyValues(key string) []model.KeyValue {
	e, ok := m.take(key)
	if !ok {
		return nil
	}
	return m.d.keyValuePairs(e)
}

// keyValuePairs reads an entry's mapping of names to scalar values.
func (d *decoder) keyValuePairs(e entry) []model.KeyValue {
	inner, ok := d.mapping(e.value, e.path, e.key)
	if !ok {
		return nil
	}
	out := make([]model.KeyValue, 0, len(inner.entries))
	for _, ie := range inner.entries {
		out = append(out, model.KeyValue{Key: ie.key, Value: d.scalarString(ie.value, ie.path, ie.key, true)})
	}
	return out
}

// id reads the required "id" key of an entity.
func (m *mapping) id() model.ID {
	e, ok := m.take("id")
	if !ok {
		m.d.add(diag.CodeIDMissing, m.path, m.startNode(), nil)
		return ""
	}
	id := model.ID(m.d.scalarString(e.value, e.path, "id", false))
	if id == "" {
		m.d.add(diag.CodeIDMissing, e.path, e.value, nil)
		return ""
	}
	if !ids.Valid(id) {
		m.d.add(diag.CodeIDInvalid, e.path, e.value, map[string]string{"id": string(id)})
	}
	return id
}

func (m *mapping) meta() model.Meta {
	return model.Meta{ID: m.id(), Name: m.str("name"), Description: m.str("description")}
}

func (m *mapping) child(key string) (*mapping, bool) {
	e, ok := m.take(key)
	if !ok {
		return nil, false
	}
	return m.d.mapping(e.value, e.path, key)
}

func (m *mapping) ref(key string) (model.Ref, bool) {
	rm, ok := m.child(key)
	if !ok {
		return model.Ref{}, false
	}
	if r, isParam := rm.paramRef(); isParam {
		return r, true
	}
	r := model.Ref{Collection: rm.str("collection"), ID: rm.id(), Path: rm.str("path")}
	rm.closeStrict()
	return r, true
}

// paramRef reads {param: name}, which only a blueprint may use. Any other
// key beside it is reported as unknown.
func (m *mapping) paramRef() (model.Ref, bool) {
	if !m.d.blueprint {
		return model.Ref{}, false
	}
	e, ok := m.take("param")
	if !ok {
		return model.Ref{}, false
	}
	r := model.Ref{Param: m.d.scalarString(e.value, e.path, "param", false)}
	m.closeStrict()
	return r, true
}

func encodeRef(r model.Ref) any {
	var o omap
	if r.Param != "" {
		o.put("param", r.Param)
		return o.items
	}
	o.put("collection", r.Collection)
	o.put("id", string(r.ID))
	o.put("path", r.Path)
	return o.items
}

// versionLabel reads a version label from its literal text, so unquoted
// `version: 1.10` stays "1.10" instead of failing as a number.
func (m *mapping) versionLabel(key string) string {
	e, ok := m.take(key)
	if !ok {
		return ""
	}
	return m.d.scalarString(e.value, e.path, key, true)
}
