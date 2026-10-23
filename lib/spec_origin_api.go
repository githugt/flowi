// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"path/filepath"
	"slices"

	"github.com/githugt/flowi-lib/model"
)

// SpecOrigin is the kept spec a request comes from and the fields of a
// version its overlays set, so the window can mark them.
type SpecOrigin struct {
	Spec   string // the kept spec, relative to the collection root
	Name   string // its name, for PreviewSpecUpdate
	Fields []OverlayField
}

// OverlayField is a field of a request version an overlay sets.
type OverlayField struct {
	Section string // docs, body, query or header
	Key     string // the parameter's name; "" for docs and body
	File    string // the overlay that sets it last, relative to the collection root
}

// overlayLayer is one overlay as it applies to a kept spec.
type overlayLayer struct {
	rel  string
	data []byte
}

// SpecOrigin says which kept spec makes the request's version label and
// what its overlays set; ErrNotFromSpec when none makes it.
func (w *Workspace) SpecOrigin(t RequestTarget, label string) (SpecOrigin, error) {
	c, req, err := w.requestFor(t)
	if err != nil {
		return SpecOrigin{}, err
	}
	m, err := findSpec(c.Root, req.Path, label)
	if err != nil {
		return SpecOrigin{}, err
	}
	layers, err := m.layers()
	if err != nil {
		return SpecOrigin{}, err
	}
	out := SpecOrigin{Spec: m.kept.Rel, Name: m.kept.Name}
	setBy := map[OverlayField]string{} // field, file left empty → the file that set it last
	var order []OverlayField
	prev, prevOK := versionAfter(m, layers[:0], req.Path, label)
	for i, l := range layers {
		cur, ok := versionAfter(m, layers[:i+1], req.Path, label)
		if ok && prevOK {
			for _, f := range changedFields(prev, cur) {
				if _, seen := setBy[f]; !seen {
					order = append(order, f)
				}
				setBy[f] = l.rel
			}
		}
		prev, prevOK = cur, ok // a request an overlay first makes (an added operationId) has nothing to compare with
	}
	for _, f := range order {
		f.File = setBy[f]
		out.Fields = append(out.Fields, f)
	}
	return out, nil
}

// layers are the kept overlays and then the edits overlay, as files.
func (m specMatch) layers() ([]overlayLayer, error) {
	out := make([]overlayLayer, 0, len(m.overlays)+1)
	for i, data := range m.overlays {
		out = append(out, overlayLayer{rel: "descriptions/" + filepath.Base(m.kept.Overlays[i]), data: data})
	}
	if m.edits != nil {
		data, err := encodeOverlay(m.edits.Actions)
		if err != nil {
			return nil, err
		}
		out = append(out, overlayLayer{rel: "descriptions/" + filepath.Base(m.kept.Edits), data: data})
	}
	return out, nil
}

// versionAfter is the request's version once the given layers apply.
func versionAfter(m specMatch, layers []overlayLayer, requestPath, label string) (model.RequestVersion, bool) {
	overlays := make([][]byte, len(layers))
	for i, l := range layers {
		overlays[i] = l.data
	}
	only := m
	only.overlays = overlays
	doc, err := only.overlaid(nil)
	if err != nil {
		return model.RequestVersion{}, false
	}
	v, _, err := versionIn(doc, requestPath, label)
	return v, err == nil
}

// changedFields are the fields of a version an overlay changed: the docs,
// the body, and parameters it added.
func changedFields(before, after model.RequestVersion) []OverlayField {
	var out []OverlayField
	if before.Docs != after.Docs {
		out = append(out, OverlayField{Section: "docs"})
	}
	if !sameBody(before.Body, after.Body) {
		out = append(out, OverlayField{Section: "body"})
	}
	for _, kind := range []struct {
		section       string
		before, after []model.KeyValue
	}{{"query", before.Query, after.Query}, {"header", before.Headers, after.Headers}} {
		for _, p := range kind.after {
			if !slices.ContainsFunc(kind.before, func(b model.KeyValue) bool { return b.Key == p.Key }) {
				out = append(out, OverlayField{Section: kind.section, Key: p.Key})
			}
		}
	}
	return out
}
