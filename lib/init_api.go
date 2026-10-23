// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"github.com/githugt/flowi-lib/internal/migrate"
	"github.com/githugt/flowi-lib/model"
)

// InitCollection creates a new, empty collection in dir and optionally
// mounts it.
func (w *Workspace) InitCollection(dir, name string, mount bool) (model.Manifest, error) {
	return w.initCollection(dir, name, w.newID(), mount)
}

func (w *Workspace) initCollection(dir, name string, id model.ID, mount bool) (model.Manifest, error) {
	if mount {
		if err := w.workspaceWritable(); err != nil {
			return model.Manifest{}, err // refuse before creating anything
		}
	}
	m := model.Manifest{Format: migrate.CurrentFormat, Meta: model.Meta{ID: id, Name: name}}
	if err := w.store.Init(dir, m); err != nil {
		return model.Manifest{}, err
	}
	if !mount {
		return m, nil
	}
	return w.Mount(dir)
}

func (w *Workspace) workspaceWritable() error {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if len(w.modelProblems) > 0 {
		return w.modelProblems.Err()
	}
	return nil
}
