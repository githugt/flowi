// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/model"
)

// A request's working copy (its dirty edit, see StoreEdit) is kept in the workspace's data directory, one file per
// request id, and not in the request file. The app saves it every few seconds while someone types, so when it lived
// in the request file a `git add -A` committed half-typed headers and tokens, and every autosave touched a tracked
// file. The file is private to the person (mode 0600), follows the request when it is moved or renamed (the id does
// not change), and a collection cloned from someone else never carries one.
// Earlier releases wrote the edit into the request file as `edit:`. Loading still reads that, moves it to the data
// directory, and the next write of the request file leaves it out.

const editsDirName = "edits"

func (w *Workspace) editsDir(collection model.ID) string {
	return filepath.Join(w.DataDir(collection), editsDirName)
}

func (w *Workspace) editFile(collection, request model.ID) string {
	return filepath.Join(w.editsDir(collection), string(request)+".yaml")
}

// withEdits gives c's requests the working copies kept in the data directory. A request whose file still carries an
// edit of its own (written by an earlier release) has it moved there first, unless a kept one exists, which wins.
// A kept file that cannot be read or understood is left alone and the request has no edit: the person loses a
// draft, never the saved request.
func (w *Workspace) withEdits(c model.Collection) model.Collection {
	dir := w.editsDir(c.Manifest.ID)
	have := map[string]bool{}
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if name, ok := strings.CutSuffix(e.Name(), ".yaml"); ok && !e.IsDir() {
				have[name] = true
			}
		}
	}
	reqs := slices.Clone(c.Requests)
	for i, r := range reqs {
		switch {
		case have[string(r.ID)]:
			reqs[i] = w.readEdit(c.Manifest.ID, r)
		case r.Edit != nil:
			_ = w.writeEdit(c.Manifest.ID, r) // best effort: the edit is in memory either way
		}
	}
	c.Requests = reqs
	return c
}

// readEdit is r with the kept working copy, or r with none when the file does not read.
func (w *Workspace) readEdit(collection model.ID, r model.Request) model.Request {
	file := w.editFile(collection, r.ID)
	data, err := os.ReadFile(file)
	if err != nil {
		r.Edit, r.EditBase, r.EditOverlay = nil, "", ""
		return r
	}
	v, base, overlay, diags := w.store.DecodeEdit(file, data)
	if v == nil || diags.HasErrors() {
		r.Edit, r.EditBase, r.EditOverlay = nil, "", ""
		return r
	}
	r.Edit, r.EditBase, r.EditOverlay = v, base, overlay
	return r
}

// writeEdit keeps r's working copy, or removes the kept one when r has none.
func (w *Workspace) writeEdit(collection model.ID, r model.Request) error {
	file := w.editFile(collection, r.ID)
	data, err := w.store.EncodeEdit(r)
	if err != nil {
		return fmt.Errorf("encoding the working copy of %s: %w", r.Path, err)
	}
	if data == nil {
		if err := os.Remove(file); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("removing %s: %w", file, err)
		}
		return nil
	}
	return writePrivate(file, data)
}

// writePrivate replaces file with data atomically, readable by the owner only: a working copy may hold a token that
// was typed and never saved.
func writePrivate(file string, data []byte) error {
	dir := filepath.Dir(file)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".flowi-*")
	if err != nil {
		return fmt.Errorf("creating a temporary file in %s: %w", dir, err)
	}
	defer os.Remove(tmp.Name()) // fails harmlessly once renamed
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr, os.Chmod(tmp.Name(), 0o600)); err != nil {
		return fmt.Errorf("writing %s: %w", file, err)
	}
	if err := os.Rename(tmp.Name(), file); err != nil {
		return fmt.Errorf("replacing %s: %w", file, err)
	}
	return nil
}

// keepEdit keeps r's working copy (removing it when r has none) and shows it on the loaded request. The request
// file is not touched.
func (w *Workspace) keepEdit(collection model.ID, r model.Request) error {
	if err := w.writeEdit(collection, r); err != nil {
		return err
	}
	w.setEdit(collection, r)
	return nil
}

// setEdit puts r's working copy on the loaded request with the same id.
func (w *Workspace) setEdit(collection model.ID, r model.Request) {
	w.mu.Lock()
	defer w.mu.Unlock()
	ci := slices.IndexFunc(w.collections, func(c model.Collection) bool { return c.Manifest.ID == collection })
	if ci < 0 {
		return
	}
	reqs := slices.Clone(w.collections[ci].Requests)
	ri := slices.IndexFunc(reqs, func(x model.Request) bool { return x.ID == r.ID })
	if ri < 0 {
		return
	}
	reqs[ri].Edit, reqs[ri].EditBase, reqs[ri].EditOverlay = r.Edit, r.EditBase, r.EditOverlay
	w.collections[ci].Requests = reqs
}

// sameEdit says whether a and b hold the same working copy.
func sameEdit(a, b model.Request) bool {
	return a.EditBase == b.EditBase && a.EditOverlay == b.EditOverlay && reflect.DeepEqual(a.Edit, b.Edit)
}
