// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/specoverlay"
	"github.com/githugt/flowi/lib/store"
)

// ExportItem is an API description a collection can hand to someone outside Flowi.
type ExportItem struct {
	ID    string // what ExportDescription takes
	Label string // in words, for a list
	File  string // the file name to suggest
}

// The ids of the two descriptions Flowi makes itself; a kept one is "kept:<name>".
const (
	ExportGenerated      = "generated"
	ExportGeneratedAsync = "generated-async"
	exportKeptPrefix     = "kept:"
)

// ErrNoExport is returned for an id ExportItems does not list.
var ErrNoExport = errors.New("this collection has no such description to export")

// ExportItems lists what a collection can export: its requests as Flowi sends them (the generated OpenAPI
// description, one operation per request, and the AsyncAPI one when it has message requests), and each OpenAPI,
// AsyncAPI or OpenRPC description it kept at import as it stands now, with its overlays and Flowi's own edits laid
// over it. Nothing is written.
func (w *Workspace) ExportItems(collection model.ID) ([]ExportItem, error) {
	root, err := w.collectionRoot(collection)
	if err != nil {
		return nil, err
	}
	var out []ExportItem
	if exists(filepath.Join(root, filepath.FromSlash(store.GeneratedDescription))) {
		out = append(out, ExportItem{ID: ExportGenerated, Label: "The requests as Flowi sends them (OpenAPI)", File: "requests.openapi.yaml"})
	}
	if exists(filepath.Join(root, filepath.FromSlash(store.GeneratedAsyncDescription))) {
		out = append(out, ExportItem{ID: ExportGeneratedAsync, Label: "The message requests as Flowi sends them (AsyncAPI)", File: "requests.asyncapi.yaml"})
	}
	for _, l := range lintables(root) {
		if l.proto {
			continue // already a file of its own
		}
		out = append(out, ExportItem{ID: exportKeptPrefix + filepath.Base(l.file), Label: l.name + " as it stands, with its overlays applied", File: filepath.Base(l.file)})
	}
	return out, nil
}

func exists(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}

// ExportDescription returns the text of one item of ExportItems: the generated description as it is on disk (it is
// rewritten after every change to a request, a flow or a learned schema), or a kept description with its overlays
// applied. A kept description whose overlays do not apply cannot be exported, and the error says so.
func (w *Workspace) ExportDescription(collection model.ID, id string) ([]byte, error) {
	root, err := w.collectionRoot(collection)
	if err != nil {
		return nil, err
	}
	switch {
	case id == ExportGenerated:
		return os.ReadFile(filepath.Join(root, filepath.FromSlash(store.GeneratedDescription)))
	case id == ExportGeneratedAsync:
		return os.ReadFile(filepath.Join(root, filepath.FromSlash(store.GeneratedAsyncDescription)))
	case strings.HasPrefix(id, exportKeptPrefix):
		base := strings.TrimPrefix(id, exportKeptPrefix)
		for _, l := range lintables(root) {
			if l.proto || filepath.Base(l.file) != base {
				continue
			}
			data, overlays, err := l.read()
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", l.name, err)
			}
			out, diags := specoverlay.Apply(data, overlays)
			if diags.HasErrors() {
				return nil, diags.Err()
			}
			return out, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrNoExport, id)
}
