// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"fmt"
	"slices"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/store"
)

// Format returns the canonical-formatting changes for the collection in dir,
// or for every mounted collection when dir is "". Path hints are repaired
// across collections. Files with problems that a rewrite would turn into
// data loss (see BlocksRewrite) are left out; their diagnostics are
// returned as blocked. Nothing is written; pass changes to ApplyChanges.
func (w *Workspace) Format(dir string) (changes []FileChange, blocked diag.List, err error) {
	targets, all, loadDiags, err := w.formatTargets(dir)
	if err != nil {
		return nil, nil, err
	}
	blocked = blocking(loadDiags)
	refused := map[string]bool{}
	for _, d := range blocked {
		refused[d.File] = true
	}
	for _, c := range targets {
		repaired := store.RepairRefs(c, globalResolver(c, all))
		cs, err := w.store.Format(repaired)
		if err != nil {
			return nil, nil, err
		}
		changes = append(changes, slices.DeleteFunc(cs, func(ch FileChange) bool { return refused[ch.File] })...)
	}
	return changes, blocked, nil
}

// formatTargets returns the collections to format, every collection for
// resolving references, and the load diagnostics of the targets.
func (w *Workspace) formatTargets(dir string) (targets, all []model.Collection, loadDiags diag.List, err error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if dir == "" {
		return slices.Clone(w.collections), w.collections, w.positioned(w.loadDiags), nil
	}
	c, cdiags, err := w.store.Load(dir)
	if errors.Is(err, store.ErrNotCollection) {
		return nil, nil, nil, diag.List{notCollection(dir)}.Err()
	}
	if err != nil {
		return nil, nil, nil, fmt.Errorf("loading %s: %w", dir, err)
	}
	return []model.Collection{c}, withCollection(w.collections, c), w.positioned(cdiags), nil
}

// globalResolver resolves references from c, following its dependencies
// into the other loaded collections.
func globalResolver(c model.Collection, all []model.Collection) store.RefResolver {
	byID := map[model.ID]model.Collection{}
	for _, x := range all {
		byID[x.Manifest.ID] = x
	}
	return func(r model.Ref) (string, bool) {
		target := c
		if r.Collection != "" {
			dep, ok := dependency(c, r.Collection)
			if !ok {
				return "", false
			}
			if target, ok = byID[dep]; !ok {
				return "", false
			}
		}
		local := r
		local.Collection = ""
		return store.LocalResolver(target)(local)
	}
}

func dependency(c model.Collection, name string) (model.ID, bool) {
	for _, d := range c.Manifest.DependsOn {
		if d.Name == name || string(d.ID) == name {
			return d.ID, true
		}
	}
	return "", false
}

// ApplyChanges writes formatting changes atomically.
func ApplyChanges(changes []FileChange) error {
	return store.Apply(changes)
}
