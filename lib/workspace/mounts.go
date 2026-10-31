// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

// Mount errors.
var (
	ErrAlreadyMounted = errors.New("collection already mounted")
	ErrNotMounted     = errors.New("collection not mounted")
)

// PathResolver turns a mount path into a comparable form (absolute, with
// symlinks resolved). Injected so tests need no real filesystem.
type PathResolver func(path string) (string, error)

// Mount returns a copy of ws with the collection added. The path is kept
// as given (so "~" survives), but duplicates are detected on the resolved
// path.
func Mount(ws model.Workspace, id model.ID, path string, resolve PathResolver) (model.Workspace, error) {
	path = filepath.Clean(path)
	target, err := resolve(path)
	if err != nil {
		return ws, fmt.Errorf("resolving %s: %w", path, err)
	}
	for _, m := range ws.Collections {
		existing, resolveErr := resolve(m.Path)
		if m.ID == id || (resolveErr == nil && existing == target) {
			return ws, fmt.Errorf("%s: %w", path, ErrAlreadyMounted)
		}
	}
	ws.Collections = append(slices.Clone(ws.Collections), model.Mount{ID: id, Path: path})
	return ws, nil
}

// Unmount returns a copy of ws without the collection.
func Unmount(ws model.Workspace, id model.ID) (model.Workspace, error) {
	i := slices.IndexFunc(ws.Collections, func(m model.Mount) bool { return m.ID == id })
	if i < 0 {
		return ws, fmt.Errorf("%s: %w", id, ErrNotMounted)
	}
	ws.Collections = slices.Delete(slices.Clone(ws.Collections), i, i+1)
	return ws, nil
}

// Relocate returns a copy of ws with the collection's path changed.
func Relocate(ws model.Workspace, id model.ID, path string) (model.Workspace, error) {
	i := slices.IndexFunc(ws.Collections, func(m model.Mount) bool { return m.ID == id })
	if i < 0 {
		return ws, fmt.Errorf("%s: %w", id, ErrNotMounted)
	}
	ws.Collections = slices.Clone(ws.Collections)
	ws.Collections[i].Path = filepath.Clean(path)
	return ws, nil
}

// CheckMounts reports every mount whose directory does not exist.
func CheckMounts(ws model.Workspace, home string, exists func(string) bool) diag.List {
	var out diag.List
	for i, m := range ws.Collections {
		if exists(ExpandPath(m.Path, home)) {
			continue
		}
		loc := diag.Location{File: ws.File, Path: fmt.Sprintf("$.collections[%d].path", i)}
		out = append(out, diag.New(diag.CodeWSMissing, loc, map[string]string{"id": string(m.ID), "path": m.Path}))
	}
	return out
}
