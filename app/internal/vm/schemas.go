// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"encoding/json"
	"fmt"

	flowi "github.com/githugt/flowi/lib"
)

// SchemaItem is one stored schema of the selected version: its title, where
// it came from, and its JSON, pretty-printed.
type SchemaItem struct {
	Part   flowi.SchemaPart
	Source flowi.SchemaSource
	Title  string
	Text   string
}

// Schemas is the schemas learned from (or imported for) the selected version
// of a request, read in the background. Reload them after each send: a send
// may have learned or widened one.
type Schemas struct {
	b        Backend
	w        *Worker
	req      *Request
	items    []SchemaItem
	onChange listeners
}

// NewSchemas returns the schemas view of req; call Load.
func NewSchemas(b Backend, w *Worker, req *Request) *Schemas {
	return &Schemas{b: b, w: w, req: req}
}

// OnChange registers f to run after the items change. It returns the
// function that removes f again.
func (s *Schemas) OnChange(f func()) (remove func()) { return s.onChange.add(f) }

// Items are the stored schemas: the request body first, then the responses
// by status.
func (s *Schemas) Items() []SchemaItem { return s.items }

// Any reports whether the version has a schema (the panel's dot).
func (s *Schemas) Any() bool { return len(s.items) > 0 }

// Load reads the schemas of the selected version in the background.
func (s *Schemas) Load() {
	t, version := s.req.Target(), s.req.SelectedLabel() // read here: the worker must not touch the request
	Run(s.w, func() []SchemaItem { return s.read(t, version) }, func(items []SchemaItem) {
		s.items = items
		s.onChange.fire()
	})
}

// read runs off the UI goroutine.
func (s *Schemas) read(t flowi.RequestTarget, version string) []SchemaItem {
	var out []SchemaItem
	for _, part := range s.b.ListSchemas(t, version) {
		schema, source, ok := s.b.SchemaFor(t, version, part)
		if !ok {
			continue
		}
		text, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			continue
		}
		out = append(out, SchemaItem{Part: part, Source: source, Title: schemaTitle(part), Text: string(text)})
	}
	return out
}

func schemaTitle(p flowi.SchemaPart) string {
	if p.Kind == "request" {
		return "Request body"
	}
	return fmt.Sprintf("Response %d", p.Status)
}

// Delete removes a schema; the next send learns it again. Schemas from a spec
// are removed too: the owner asked for it.
func (s *Schemas) Delete(item SchemaItem) {
	t, version := s.req.Target(), s.req.SelectedLabel()
	Run(s.w, func() []SchemaItem {
		_ = s.b.DeleteSchema(t, version, item.Part) // a failure shows as the schema still being listed after the reload
		return s.read(t, version)
	}, func(items []SchemaItem) {
		s.items = items
		s.onChange.fire()
	})
}
