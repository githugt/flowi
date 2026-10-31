// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

// DuplicateRequest copies request t to path in the same collection, with a
// fresh id and without the original's unsaved edit. Path rules are as for
// NewRequest (FLW-WRITE-002); an unknown request is FLW-REQ-001.
func (w *Workspace) DuplicateRequest(t RequestTarget, path string) (RequestTarget, error) {
	return w.DuplicateRequestTo(t, t.Collection, path)
}

// DuplicateRequestTo is DuplicateRequest into collection to, which may be
// t's own: the original stays where it is. An unknown collection is
// FLW-WS-005.
func (w *Workspace) DuplicateRequestTo(t RequestTarget, to model.ID, path string) (RequestTarget, error) {
	_, r, err := w.requestFor(t)
	if err != nil {
		return RequestTarget{}, err
	}
	dest, ok := w.loadedCollection(to)
	if !ok {
		return RequestTarget{}, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(to)})
	}
	if detail := requestPathProblem(path); detail != "" {
		return RequestTarget{}, diagErr(diag.CodeWriteRequestPath, map[string]string{"path": path, "detail": detail})
	}
	file := filepath.Join(dest.Root, "requests", filepath.FromSlash(path)+".req.yaml")
	if err := requireFree(file, path); err != nil {
		return RequestTarget{}, err
	}
	copied := renamedRequest(r, path, file)
	copied.ID, copied.Edit = w.newID(), nil
	data, err := w.store.EncodeRequest(copied)
	if err != nil {
		return RequestTarget{}, fmt.Errorf("encoding %s: %w", file, err)
	}
	if err := w.writeAndReload(to, file, data, SaveOptions{}); err != nil {
		return RequestTarget{}, err
	}
	return RequestTarget{Collection: to, Request: copied.ID}, nil
}

// DeleteRequest removes request t's file. Flows that use it are not
// rewritten; Validate reports them. An unknown request is FLW-REQ-001.
func (w *Workspace) DeleteRequest(t RequestTarget) error {
	_, r, err := w.requestFor(t)
	if err != nil {
		return err
	}
	return w.removeFile(t.Collection, r.File)
}

// DuplicateFlow copies flow t to path in the same collection, with a fresh
// id. Path rules are as for NewFlow (FLW-WRITE-002); an unknown flow is
// FLW-FLOW-002. The canvas layout is not copied: the copy lays itself out.
func (w *Workspace) DuplicateFlow(t FlowTarget, path string) (FlowTarget, error) {
	c, f, err := w.flowFor(t)
	if err != nil {
		return FlowTarget{}, err
	}
	if detail := requestPathProblem(path); detail != "" {
		return FlowTarget{}, diagErr(diag.CodeWriteRequestPath, map[string]string{"path": path, "detail": detail})
	}
	file := filepath.Join(c.Root, "flows", filepath.FromSlash(path)+flowSuffix)
	if err := requireFree(file, path); err != nil {
		return FlowTarget{}, err
	}
	copied := f
	copied.ID, copied.Path, copied.File = w.newID(), path, file
	if f.Name == "" || f.Name == filepath.Base(f.Path) {
		copied.Name = filepath.Base(path)
	}
	data, err := w.store.EncodeFlow(c, copied)
	if err != nil {
		return FlowTarget{}, fmt.Errorf("encoding %s: %w", file, err)
	}
	if err := w.writeAndReload(t.Collection, file, data, SaveOptions{}); err != nil {
		return FlowTarget{}, err
	}
	return FlowTarget{Collection: t.Collection, Flow: copied.ID}, nil
}

// DeleteFlow removes flow t's file and its canvas layout. An unknown flow
// is FLW-FLOW-002.
func (w *Workspace) DeleteFlow(t FlowTarget) error {
	_, f, err := w.flowFor(t)
	if err != nil {
		return err
	}
	// The layout goes first: if it cannot be removed the flow is still
	// there and the error says nothing was deleted.
	if err := w.removeFile(t.Collection, layoutFile(f.File)); err != nil {
		return err
	}
	return w.removeFile(t.Collection, f.File)
}

// removeFile deletes file (already gone is fine) and refreshes the
// collection's view of it.
func (w *Workspace) removeFile(collection model.ID, file string) error {
	if err := os.Remove(file); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("removing %s: %w", file, err)
	}
	return w.reloadFile(collection, file)
}
