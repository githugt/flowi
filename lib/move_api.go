// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/store"
)

// MoveRequest moves request t to path (slash-separated, without suffix) in
// collection to, which may be its own collection (a rename). The request
// keeps its id, versions and dirty edit. A destination that exists, or a bad
// path, is FLW-WRITE-002 and nothing moves; an unknown collection is
// FLW-WS-005. Flows that refer to the request by path or collection are not
// rewritten (a request leaving its collection breaks them; Validate says so).
func (w *Workspace) MoveRequest(t RequestTarget, to model.ID, path string) (RequestTarget, error) {
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
	moved := renamedRequest(r, path, file)
	data, err := w.store.EncodeRequest(moved)
	if err != nil {
		return RequestTarget{}, fmt.Errorf("encoding %s: %w", file, err)
	}
	if err := store.Apply([]store.FileChange{{File: file, New: data}}); err != nil {
		return RequestTarget{}, err
	}
	if err := os.Remove(r.File); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return RequestTarget{}, fmt.Errorf("removing the old %s: %w", r.File, err)
	}
	if err := w.reloadFile(t.Collection, r.File); err != nil {
		return RequestTarget{}, err
	}
	if err := w.reloadFile(to, file); err != nil {
		return RequestTarget{}, err
	}
	return RequestTarget{Collection: to, Request: r.ID}, nil
}

// requireFree refuses a destination file that exists.
func requireFree(file, path string) error {
	_, err := os.Stat(file)
	switch {
	case err == nil:
		return diagErr(diag.CodeWriteRequestPath, map[string]string{"path": path, "detail": "it already exists"})
	case !errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("checking %s: %w", file, err)
	}
	return nil
}

// renamedRequest is r at its new path and file. A name that was only the
// old file name (the default) follows the new one.
func renamedRequest(r model.Request, path, file string) model.Request {
	if r.Name == "" || r.Name == filepath.Base(r.Path) {
		r.Name = filepath.Base(path)
	}
	r.Path, r.File = path, file
	r.Versions = slices.Clone(r.Versions)
	return r
}

// folderDir resolves folder path under tree ("requests" or "flows") of
// collection to its directory. A bad tree or path is FLW-WRITE-002.
func (w *Workspace) folderDir(collection model.ID, tree, path string) (model.Collection, string, error) {
	c, ok := w.loadedCollection(collection)
	if !ok {
		return model.Collection{}, "", diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	fail := func(detail string) error {
		return diagErr(diag.CodeWriteRequestPath, map[string]string{"path": path, "detail": detail})
	}
	if tree != "requests" && tree != "flows" {
		return c, "", fail("folders go under requests or flows")
	}
	if detail := requestPathProblem(path); detail != "" {
		return c, "", fail(detail)
	}
	return c, filepath.Join(c.Root, tree, filepath.FromSlash(path)), nil
}

// MoveFolder moves the folder from to the path to under tree ("requests" or
// "flows") of collection, with everything in it: requests keep their ids,
// versions and dirty edits, since a request's path is its place on disk. A
// destination that exists, a move into the folder itself, or a bad path is
// FLW-WRITE-002 and nothing moves. Flows that refer to the requests by path
// are not rewritten (Validate says so).
func (w *Workspace) MoveFolder(collection model.ID, tree, from, to string) error {
	c, src, err := w.folderDir(collection, tree, from)
	if err != nil {
		return err
	}
	_, dst, err := w.folderDir(collection, tree, to)
	if err != nil {
		return err
	}
	fail := func(detail string) error {
		return diagErr(diag.CodeWriteRequestPath, map[string]string{"path": to, "detail": detail})
	}
	if to == from || strings.HasPrefix(to, from+"/") {
		return fail("a folder cannot be moved into itself")
	}
	if info, err := os.Stat(src); err != nil || !info.IsDir() {
		return diagErr(diag.CodeWriteRequestPath, map[string]string{"path": from, "detail": "there is no such folder"})
	}
	if err := requireFree(dst, to); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil { //nolint:gosec // collection directories are normally world-readable
		return fmt.Errorf("creating %s: %w", filepath.Dir(dst), err)
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("moving %s to %s: %w", src, dst, err)
	}
	_, err = w.Reload(Change{Root: c.Root, File: c.Root, Op: store.ChangeRescan})
	return err
}

// DeleteFolder removes the folder path under tree of collection and
// everything in it. Callers ask first when it holds anything.
func (w *Workspace) DeleteFolder(collection model.ID, tree, path string) error {
	c, dir, err := w.folderDir(collection, tree, path)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("removing %s: %w", dir, err)
	}
	_, err = w.Reload(Change{Root: c.Root, File: c.Root, Op: store.ChangeRescan})
	return err
}
