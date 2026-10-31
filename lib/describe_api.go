// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/githugt/flowi/lib/arazzo"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/store"
)

// RegenerateDescription rewrites the collection's generated OpenAPI
// description (descriptions/requests.openapi.yaml): one operation per request
// version, the schemas learned for them, and the built-in operation custom
// flow steps call. It is derived, so it is rewritten only when its bytes
// change (a commit sees no noise), and removed when the collection has
// neither requests nor flows. It runs after every change to a request, a flow
// or a learned schema, and when a collection is mounted.
func (w *Workspace) RegenerateDescription(collection model.ID) error {
	c, ok := w.loadedCollection(collection)
	if !ok {
		return nil
	}
	file := filepath.Join(c.Root, filepath.FromSlash(store.GeneratedDescription))
	if len(c.Requests) == 0 && len(c.Flows) == 0 {
		return errors.Join(writeDerived(file, nil), writeDerived(filepath.Join(c.Root, filepath.FromSlash(store.GeneratedAsyncDescription)), nil))
	}
	lookup := schemaLookup{w: w, collection: collection}
	data, err := arazzo.Describe(c, lookup)
	if err != nil {
		return err
	}
	if err := writeDerived(file, data); err != nil {
		return err
	}
	asyncData, err := arazzo.DescribeAsync(c, lookup)
	if err != nil {
		return err
	}
	return writeDerived(filepath.Join(c.Root, filepath.FromSlash(store.GeneratedAsyncDescription)), asyncData)
}

// writeDerived writes a derived file only when its bytes change, and removes
// it when there is nothing to say (data is nil).
func writeDerived(file string, data []byte) error {
	if data == nil {
		if err := os.Remove(file); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("removing %s: %w", file, err)
		}
		return nil
	}
	if old, readErr := os.ReadFile(file); readErr == nil && bytes.Equal(old, data) {
		return nil
	}
	return store.Apply([]store.FileChange{{File: file, New: data}})
}

// describesFile reports whether a change to file can change the description:
// a request or a flow of the collection.
func (w *Workspace) describesFile(collection model.ID, file string) bool {
	c, ok := w.loadedCollection(collection)
	if !ok {
		return false
	}
	rel, err := filepath.Rel(c.Root, file)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	return strings.HasPrefix(rel, "requests/") || strings.HasPrefix(rel, "flows/")
}

// schemaLookup gives arazzo.Describe the stored schemas of a collection's requests.
type schemaLookup struct {
	w          *Workspace
	collection model.ID
}

func (s schemaLookup) RequestSchema(r model.Request, version string) (map[string]any, bool) {
	schema, _, ok := s.w.SchemaFor(RequestTarget{Collection: s.collection, Request: r.ID}, version, SchemaPart{Kind: "request"})
	return schema, ok
}

func (s schemaLookup) ResponseSchemas(r model.Request, version string) map[int]map[string]any {
	t := RequestTarget{Collection: s.collection, Request: r.ID}
	out := map[int]map[string]any{}
	for _, part := range s.w.ListSchemas(t, version) {
		if part.Kind != "response" {
			continue
		}
		if schema, _, ok := s.w.SchemaFor(t, version, part); ok {
			out[part.Status] = schema
		}
	}
	return out
}

func (s schemaLookup) MessageSchema(r model.Request, version string) (map[string]any, bool) {
	schema, _, ok := s.w.SchemaFor(RequestTarget{Collection: s.collection, Request: r.ID}, version, SchemaPart{Kind: "message"})
	return schema, ok
}
