// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/store"
	"github.com/githugt/flowi-lib/validate"
)

// Validate returns every problem in the workspace: load diagnostics,
// cross-file validation and contract warnings, positioned and sorted.
func (w *Workspace) Validate() diag.List {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.positioned(slices.Concat(w.structuralDiags(), w.contractWarnings(w.collections)))
}

// validateStructure is Validate without the contract phase.
func (w *Workspace) validateStructure() diag.List {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.positioned(w.structuralDiags())
}

// structuralDiags are load diagnostics and cross-file validation, without
// the contract phase: what decides whether a send or run may start.
// Callers hold mu.
func (w *Workspace) structuralDiags() diag.List {
	return slices.Concat(w.loadDiags, validate.Validate(validate.Target{Collections: w.collections}), w.scheduleDiags())
}

// ValidateDir validates the collection in dir, mounted or not, with the
// mounted collections available as dependencies. Only problems in dir's
// own files are returned.
func (w *Workspace) ValidateDir(dir string) (diag.List, error) {
	c, loadDiags, err := w.store.Load(dir)
	if errors.Is(err, store.ErrNotCollection) {
		return nil, diag.List{notCollection(dir)}.Err()
	}
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", dir, err)
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	target := withCollection(w.collections, c)
	all := slices.Concat(loadDiags, validate.Validate(validate.Target{Collections: target}), w.contractWarnings(target))
	inDir := slices.DeleteFunc(all, func(d diag.Diagnostic) bool { return !within(c.Root, d.File) })
	return w.positioned(inDir), nil
}

// withCollection returns cs with c added, replacing any loaded copy of the
// same collection.
func withCollection(cs []model.Collection, c model.Collection) []model.Collection {
	out := slices.DeleteFunc(slices.Clone(cs), func(x model.Collection) bool { return x.Manifest.ID == c.Manifest.ID })
	return append(out, c)
}

func within(root, file string) bool {
	rel, err := filepath.Rel(root, file)
	return err == nil && !strings.HasPrefix(rel, "..")
}

// positioned fills in positions from YAML paths and sorts. Callers hold mu.
func (w *Workspace) positioned(l diag.List) diag.List {
	out := make(diag.List, 0, len(l))
	for _, d := range l {
		d = w.store.Locate(d)
		if d.Pos.Line == 0 && d.File == w.model.File && d.Path != "" {
			d = w.locateInFile(d)
		}
		out = append(out, d)
	}
	return out.Sorted()
}

// locateInFile positions diagnostics on files the store did not load, such
// as workspace.yaml.
func (w *Workspace) locateInFile(d diag.Diagnostic) diag.Diagnostic {
	data, err := os.ReadFile(d.File)
	if err != nil {
		return d // position is a convenience; the diagnostic stands without it
	}
	if pos, ok := w.codec.Locate(data, d.Path); ok {
		d.Pos = pos
	}
	return d
}
