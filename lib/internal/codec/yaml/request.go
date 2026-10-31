// SPDX-License-Identifier: Apache-2.0

package yamlcodec

import (
	"cmp"
	"slices"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/settings"
	"github.com/githugt/flowi-lib/version"
)

// DecodeRequest reads a *.req.yaml file. path is the request's path relative
// to requests/ without the suffix, e.g. "auth/login".
func (c *Codec) DecodeRequest(file, path string, data []byte) (model.Request, diag.List) {
	r := model.Request{Path: path, File: file}
	d, root, ok := c.parse(file, data)
	if !ok {
		return r, d.diags
	}
	m, ok := d.mapping(root, "$", "request")
	if !ok {
		return r, d.diags
	}
	r.Meta = m.meta()
	r.Tags = m.stringList("tags")
	r.Operation = m.str("operation")
	r.Default = m.versionLabel("default")
	r.Versions = c.versions(d, m)
	r.Edit = c.editVersion(d, m)
	r.EditBase = m.versionLabel("edit_base")
	r.EditOverlay = m.str("edit_overlay")
	r.Hooks = m.hooks()
	m.closeStrict()
	return r, d.diags
}

// versions reads the versions mapping in file order. Labels come from the
// raw key text, are validated, and must not compare equal ("1" vs "1.0").
func (c *Codec) versions(d *decoder, m *mapping) []model.RequestVersion {
	e, present := m.take("versions")
	vm, ok := d.mapping(e.value, "$.versions", "versions")
	if !present || !ok || len(vm.entries) == 0 {
		if ok {
			d.add(diag.CodeVersionNone, "$.versions", m.startNode(), nil)
		}
		return nil
	}
	var parsed []version.Label
	var labels []string
	out := make([]model.RequestVersion, 0, len(vm.entries))
	for _, ve := range vm.entries {
		label, err := version.Parse(ve.key)
		if err != nil {
			d.add(diag.CodeVersionInvalid, ve.path, ve.keyNode, map[string]string{"label": ve.key})
		} else if i := slices.IndexFunc(parsed, func(p version.Label) bool { return p.Compare(label) == 0 }); i >= 0 {
			d.add(diag.CodeVersionDuplicate, ve.path, ve.keyNode, map[string]string{"label": ve.key, "other": labels[i]})
		} else {
			parsed, labels = append(parsed, label), append(labels, ve.key)
		}
		vmap, ok := d.mapping(ve.value, ve.path, ve.key)
		if !ok {
			continue
		}
		out = append(out, c.requestVersion(d, vmap, ve.key))
	}
	return out
}

// editVersion reads the dirty edit: a version mapping without a label.
func (c *Codec) editVersion(d *decoder, m *mapping) *model.RequestVersion {
	em, ok := m.child("edit")
	if !ok {
		return nil
	}
	v := c.requestVersion(d, em, "")
	return &v
}

func (c *Codec) requestVersion(d *decoder, m *mapping, label string) model.RequestVersion {
	v := model.RequestVersion{
		Label:    label,
		Name:     m.str("name"),
		Method:   m.str("method"),
		URL:      m.str("url"),
		Query:    m.keyValues("query"),
		Headers:  m.keyValues("headers"),
		Body:     m.body(),
		Extract:  m.extractions(),
		Assert:   m.strings("assert"),
		Outcomes: m.outcomes(),
		Docs:     m.str("docs"), Variables: m.keyValues("variables"),
	}
	m.describe(&v)
	v.Auth = m.auth()
	v.Async = m.async()
	v.GRPC = m.grpc()
	v.Dataset = m.dataset()
	v.Settings = c.settingsChild(d, m, settings.LayerRequest)
	v.Hooks = m.hooks()
	m.closeStrict()
	return v
}

// auth reads the auth: block of a version, nil when there is none: its type,
// then its parameters in file order.
func (m *mapping) auth() *model.Auth {
	am, ok := m.child("auth")
	if !ok {
		return nil
	}
	a := &model.Auth{Type: am.str("type")}
	for _, e := range am.rest() {
		am.used[e.key] = true
		a.Params = append(a.Params, model.KeyValue{Key: e.key, Value: am.d.scalarString(e.value, e.path, e.key, true)})
	}
	am.closeStrict()
	return a
}

// async reads the async: block of a version, nil when there is none.
func (m *mapping) async() *model.Async {
	am, ok := m.child("async")
	if !ok {
		return nil
	}
	a := &model.Async{
		Protocol: am.str("protocol"), Server: am.str("server"), Channel: am.str("channel"),
		Binding: am.keyValues("binding"),
	}
	if sm, ok := am.child("send"); ok {
		a.Send = &model.AsyncSend{Body: sm.body(), Headers: sm.keyValues("headers")}
		sm.closeStrict()
	}
	if rm, ok := am.child("receive"); ok {
		a.Receive = &model.AsyncReceive{Count: int(rm.integer("count")), Timeout: rm.duration("timeout"), Match: rm.str("match")}
		rm.closeStrict()
	}
	am.closeStrict()
	return a
}

// grpc reads the grpc: block of a version, nil when there is none.
func (m *mapping) grpc() *model.GRPC {
	gm, ok := m.child("grpc")
	if !ok {
		return nil
	}
	g := &model.GRPC{Server: gm.str("server"), Method: gm.str("method"), Body: gm.body(), Proto: gm.strings("proto"), Import: gm.strings("import"), Web: gm.boolean("web")}
	if rm, ok := gm.child("receive"); ok {
		g.Receive = &model.AsyncReceive{Count: int(rm.integer("count")), Timeout: rm.duration("timeout"), Match: rm.str("match")}
		rm.closeStrict()
	}
	gm.closeStrict()
	return g
}

// dataset reads the dataset: block of a version, nil when there is none. Params are text whatever they are written
// as: `- 42` means the text "42", which the store reads back as a number where the column is one.
func (m *mapping) dataset() *model.Dataset {
	dm, ok := m.child("dataset")
	if !ok {
		return nil
	}
	d := &model.Dataset{Driver: dm.str("driver"), Server: dm.str("server"), Query: dm.str("query"), Limit: int(dm.integer("limit"))}
	if e, ok := dm.take("params"); ok {
		d.Params = dm.d.lenientStrings(e)
	}
	dm.closeStrict()
	return d
}

// lenientStrings reads a list of scalars as text, numbers and booleans as written.
func (d *decoder) lenientStrings(e entry) []string {
	items, ok := d.sequence(e.value, e.path, e.key)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for i, item := range items {
		out = append(out, d.scalarString(item, indexPath(e.path, i), e.key, true))
	}
	return out
}

// body reads exactly one of json, text, file, form or multipart.
func (m *mapping) body() model.Body {
	bm, ok := m.child("body")
	if !ok {
		return model.Body{}
	}
	b := model.Body{JSON: bm.str("json"), Text: bm.str("text"), File: bm.str("file"), Form: bm.keyValues("form")}
	if e, ok := bm.take("multipart"); ok {
		b.Multipart = m.d.multipartParts(e)
	}
	bm.closeStrict()
	if countBodyKinds(b) > 1 {
		m.d.typeError(bm.path, bm.startNode(), "body", "one body kind")
	}
	return b
}

// multipartParts reads the parts of a multipart body: a mapping of names to
// values, or, when a name repeats (several files sent as "files"), a list of
// one-entry mappings in order.
func (d *decoder) multipartParts(e entry) []model.Field {
	if e.value.Type() != ast.SequenceType {
		mp, ok := d.mapping(e.value, e.path, "multipart")
		if !ok {
			return nil
		}
		return d.rawFields(mp)
	}
	items, ok := d.sequence(e.value, e.path, "multipart")
	if !ok {
		return nil
	}
	var out []model.Field
	for i, item := range items {
		pm, ok := d.mapping(item, indexPath(e.path, i), "multipart")
		if !ok {
			continue
		}
		if len(pm.entries) != 1 {
			d.typeError(indexPath(e.path, i), item, "multipart", "a part with one name")
			continue
		}
		out = append(out, d.rawFields(pm)...)
	}
	return out
}

func countBodyKinds(b model.Body) int {
	kinds := []bool{b.JSON != "", b.Text != "", b.File != "", len(b.Form) > 0, len(b.Multipart) > 0}
	n := 0
	for _, set := range kinds {
		if set {
			n++
		}
	}
	return n
}

// outcomes reads the outcomes: list of a version, nil when there is none (success and failure then).
func (m *mapping) outcomes() []model.Outcome {
	e, ok := m.take("outcomes")
	if !ok {
		return nil
	}
	items, ok := m.d.sequence(e.value, e.path, "outcomes")
	if !ok || items == nil {
		return nil
	}
	out := make([]model.Outcome, 0, len(items))
	for i, item := range items {
		om, ok := m.d.mapping(item, indexPath(e.path, i), "outcomes")
		if !ok {
			continue
		}
		out = append(out, model.Outcome{Name: om.str("name"), When: om.str("when"), Disabled: om.boolean("disabled"), Color: om.str("color"), Description: om.str("description")})
		om.closeStrict()
	}
	return out
}

func encodeOutcomes(os []model.Outcome) []any {
	var out []any
	for _, x := range os {
		var o omap
		o.put("name", x.Name)
		o.put("when", x.When)
		o.put("disabled", x.Disabled)
		o.put("color", x.Color)
		o.put("description", x.Description)
		out = append(out, o.items)
	}
	return out
}

func (m *mapping) extractions() []model.Extraction {
	em, ok := m.child("extract")
	if !ok {
		return nil
	}
	out := make([]model.Extraction, 0, len(em.entries))
	for _, e := range em.entries {
		xm, ok := m.d.mapping(e.value, e.path, e.key)
		if !ok {
			continue
		}
		out = append(out, model.Extraction{
			Name: e.key, From: xm.str("from"), Path: xm.str("path"),
			Header: xm.str("header"), Cookie: xm.str("cookie"), Regex: xm.str("regex"),
		})
		xm.closeStrict()
	}
	return out
}

// EncodeRequest writes a request in canonical form, versions sorted by
// version order (invalid labels last, in text order).
func (c *Codec) EncodeRequest(r model.Request) ([]byte, error) {
	var o omap
	putMeta(&o, r.Meta)
	o.put("tags", r.Tags)
	o.put("operation", r.Operation)
	o.put("default", r.Default)
	var versions yaml.MapSlice
	for _, v := range sortedVersions(r.Versions) {
		versions = append(versions, yaml.MapItem{Key: v.Label, Value: c.encodeVersion(v)})
	}
	o.put("versions", versions)
	if r.Edit != nil {
		o.put("edit", c.encodeVersion(*r.Edit))
		o.put("edit_base", r.EditBase)
		o.put("edit_overlay", r.EditOverlay)
	}
	o.put("hooks", encodeHooks(r.Hooks))
	return marshal(o.items)
}

func sortedVersions(vs []model.RequestVersion) []model.RequestVersion {
	out := slices.Clone(vs)
	slices.SortStableFunc(out, func(a, b model.RequestVersion) int {
		la, errA := version.Parse(a.Label)
		lb, errB := version.Parse(b.Label)
		switch {
		case errA == nil && errB == nil:
			return la.Compare(lb)
		case errA == nil:
			return -1
		case errB == nil:
			return 1
		}
		return cmp.Compare(a.Label, b.Label)
	})
	return out
}

func (c *Codec) encodeVersion(v model.RequestVersion) yaml.MapSlice {
	var o omap
	o.put("name", v.Name)
	o.put("method", v.Method)
	o.put("url", v.URL)
	if v.Async != nil {
		o.putAlways("async", encodeAsync(v.Async))
	}
	o.put("query", keyValues(v.Query))
	o.put("headers", keyValues(v.Headers))
	if v.Auth != nil {
		o.putAlways("auth", encodeAuth(*v.Auth))
	}
	if v.GRPC != nil {
		o.putAlways("grpc", encodeGRPC(v.GRPC))
	}
	if v.Dataset != nil {
		o.putAlways("dataset", encodeDataset(v.Dataset))
	}
	o.put("body", encodeBody(v.Body))
	o.put("extract", encodeExtractions(v.Extract))
	o.put("assert", stringsToYAML(v.Assert))
	o.put("outcomes", encodeOutcomes(v.Outcomes))
	o.put("variables", keyValues(v.Variables))
	o.put("docs", v.Docs)
	o.put("notes", encodeNotes(v))
	o.put("settings", c.encodeSettings(v.Settings))
	o.put("hooks", encodeHooks(v.Hooks))
	// A version with nothing set is still a version: keep it as {}.
	if o.items == nil {
		return yaml.MapSlice{}
	}
	return o.items
}

func encodeBody(b model.Body) yaml.MapSlice {
	var o omap
	o.put("json", b.JSON)
	o.put("text", b.Text)
	o.put("file", b.File)
	o.put("form", keyValues(b.Form))
	o.put("multipart", multipartToYAML(b.Multipart))
	return o.items
}

func encodeExtractions(xs []model.Extraction) yaml.MapSlice {
	var out yaml.MapSlice
	for _, x := range xs {
		var o omap
		o.put("from", x.From)
		o.put("path", x.Path)
		o.put("header", x.Header)
		o.put("cookie", x.Cookie)
		o.put("regex", x.Regex)
		out = append(out, yaml.MapItem{Key: x.Name, Value: o.items})
	}
	return out
}

// describe reads the notes: the descriptions of query parameters, headers
// and variables, kept apart so those mappings stay plain name: value pairs.
func (m *mapping) describe(v *model.RequestVersion) {
	nm, ok := m.child("notes")
	if !ok {
		return
	}
	for key, list := range map[string][]model.KeyValue{"query": v.Query, "headers": v.Headers, "variables": v.Variables} {
		described := nm.keyValues(key)
		for i := range list {
			for _, n := range described {
				if n.Key == list[i].Key {
					list[i].Description = n.Value
					break // names can repeat in a list; the first one takes the note
				}
			}
		}
	}
	nm.closeStrict()
}

// encodeNotes is the notes mapping of v, empty when nothing is described.
func encodeNotes(v model.RequestVersion) yaml.MapSlice {
	var o omap
	for _, g := range []struct {
		key  string
		list []model.KeyValue
	}{{"query", v.Query}, {"headers", v.Headers}, {"variables", v.Variables}} {
		var described []model.KeyValue
		for _, kv := range g.list {
			if kv.Description != "" {
				described = append(described, model.KeyValue{Key: kv.Key, Value: kv.Description})
			}
		}
		o.put(g.key, keyValues(described))
	}
	return o.items
}

// encodeAsync is the async mapping of a version, nil for an HTTP one.
func encodeAsync(a *model.Async) yaml.MapSlice {
	if a == nil {
		return nil
	}
	var o omap
	o.put("protocol", a.Protocol)
	o.put("server", a.Server)
	o.put("channel", a.Channel)
	if a.Send != nil {
		var s omap
		s.put("body", encodeBody(a.Send.Body))
		s.put("headers", keyValues(a.Send.Headers))
		o.putAlways("send", presentMap(s.items))
	}
	if a.Receive != nil {
		var r omap
		if a.Receive.Count != 0 {
			r.put("count", a.Receive.Count)
		}
		if a.Receive.Timeout != 0 {
			r.put("timeout", a.Receive.Timeout.String())
		}
		r.put("match", a.Receive.Match)
		o.putAlways("receive", presentMap(r.items))
	}
	o.put("binding", keyValues(a.Binding))
	return presentMap(o.items)
}

// encodeGRPC is the grpc mapping of a version.
func encodeGRPC(g *model.GRPC) yaml.MapSlice {
	var o omap
	o.put("server", g.Server)
	o.put("method", g.Method)
	o.put("body", encodeBody(g.Body))
	o.put("proto", stringsToYAML(g.Proto))
	o.put("import", stringsToYAML(g.Import))
	o.put("web", g.Web)
	if g.Receive != nil {
		var r omap
		if g.Receive.Count != 0 {
			r.put("count", g.Receive.Count)
		}
		if g.Receive.Timeout != 0 {
			r.put("timeout", g.Receive.Timeout.String())
		}
		r.put("match", g.Receive.Match)
		o.putAlways("receive", presentMap(r.items))
	}
	return presentMap(o.items)
}

// encodeDataset is the dataset mapping of a version.
func encodeDataset(d *model.Dataset) yaml.MapSlice {
	var o omap
	o.put("driver", d.Driver)
	o.put("server", d.Server)
	o.put("query", d.Query)
	o.put("params", stringsToYAML(d.Params))
	if d.Limit != 0 {
		o.put("limit", d.Limit)
	}
	return presentMap(o.items)
}

// presentMap keeps an empty mapping as {} instead of dropping it: a send or
// receive with nothing set still means something.
func presentMap(items yaml.MapSlice) yaml.MapSlice {
	if items == nil {
		return yaml.MapSlice{}
	}
	return items
}

// encodeAuth writes the scheme first, then its parameters.
func encodeAuth(a model.Auth) yaml.MapSlice {
	out := yaml.MapSlice{{Key: "type", Value: a.Type}}
	for _, kv := range a.Params {
		out = append(out, yaml.MapItem{Key: kv.Key, Value: kv.Value})
	}
	return out
}
