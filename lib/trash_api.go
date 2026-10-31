// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/githugt/flowi-lib/internal/names"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/store"
	"github.com/githugt/flowi-lib/workspace"
)

// Kinds of what the trash holds.
const (
	TrashRequest = "request"
	TrashFlow    = "flow"
)

// TrashedItem is a request or flow moved out of the drafts and not deleted yet.
type TrashedItem struct {
	ID        string
	Kind      string // TrashRequest or TrashFlow
	Name      string // what it was called
	Path      string // where it was in its collection, "untitled-2"
	TrashedAt time.Time
	// Files are its files, relative to the collection root (the request or flow file, and what sat beside it).
	Files []string
}

func (w *Workspace) trashDir() string { return workspace.TrashDir(w.opts.Paths, w.opts.Name) }

// TrashStaleDrafts moves the requests and flows of the Scratch draft that have not
// been changed for olderThan to the trash, unless open says they are in use. It
// returns what it moved. Only Scratch is swept: the other drafts were made on
// purpose. A request or flow is "changed" when its file was written.
func (w *Workspace) TrashStaleDrafts(olderThan time.Duration, open func(kind string, id model.ID) bool) ([]TrashedItem, error) {
	if olderThan <= 0 {
		return nil, nil
	}
	w.draftsMu.Lock()
	defer w.draftsMu.Unlock()
	m, err := w.scratch()
	if err != nil {
		return nil, fmt.Errorf("opening Scratch: %w", err)
	}
	w.mu.RLock()
	c, ok := w.collection(m.ID)
	w.mu.RUnlock()
	if !ok {
		return nil, nil
	}
	var moved []TrashedItem
	cutoff := time.Now().Add(-olderThan)
	for _, r := range c.Requests {
		if open != nil && open(TrashRequest, r.ID) || !staleSince(r.File, cutoff) {
			continue
		}
		item, err := w.trashFiles(c, TrashRequest, r.Name, r.Path, requestFiles(c, r))
		if err != nil {
			return moved, err
		}
		moved = append(moved, item)
	}
	for _, f := range c.Flows {
		if open != nil && open(TrashFlow, f.ID) || !staleSince(f.File, cutoff) {
			continue
		}
		item, err := w.trashFiles(c, TrashFlow, f.Name, f.Path, flowFiles(c, f))
		if err != nil {
			return moved, err
		}
		moved = append(moved, item)
	}
	if len(moved) > 0 {
		if _, err := w.Reload(Change{Root: c.Root, File: c.Root, Op: store.ChangeRescan}); err != nil {
			return moved, err
		}
	}
	return moved, nil
}

// staleSince reports that file was last written before cutoff.
func staleSince(file string, cutoff time.Time) bool {
	info, err := os.Stat(file)
	return err == nil && info.ModTime().Before(cutoff)
}

// requestFiles are a request's file and what sits beside it, as paths from the collection root.
func requestFiles(c model.Collection, r model.Request) []string {
	stem := strings.TrimSuffix(r.File, ".req.yaml")
	return relativeExisting(c.Root, r.File, stem+".examples", stem+".schemas")
}

func flowFiles(c model.Collection, f model.Flow) []string {
	stem := strings.TrimSuffix(f.File, flowSuffix)
	return relativeExisting(c.Root, f.File, stem+".layout.yaml")
}

// relativeExisting are the paths among candidates that exist, relative to root.
func relativeExisting(root string, candidates ...string) []string {
	var out []string
	for _, p := range candidates {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if rel, err := filepath.Rel(root, p); err == nil {
			out = append(out, filepath.ToSlash(rel))
		}
	}
	return out
}

// trashFiles moves files (relative to the root of c) into a new trash item.
func (w *Workspace) trashFiles(c model.Collection, kind, name, path string, files []string) (TrashedItem, error) {
	item := TrashedItem{ID: string(w.newID()), Kind: kind, Name: name, Path: path, TrashedAt: time.Now().UTC(), Files: files}
	dir := filepath.Join(w.trashDir(), item.ID)
	for _, rel := range files {
		to := filepath.Join(dir, "files", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(to), 0o750); err != nil {
			return TrashedItem{}, fmt.Errorf("making room in the trash: %w", err)
		}
		if err := os.Rename(filepath.Join(c.Root, filepath.FromSlash(rel)), to); err != nil {
			return TrashedItem{}, fmt.Errorf("moving %s to the trash: %w", rel, err)
		}
	}
	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return TrashedItem{}, fmt.Errorf("describing the trashed item: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "item.json"), append(data, '\n'), 0o600); err != nil {
		return TrashedItem{}, fmt.Errorf("describing the trashed item: %w", err)
	}
	return item, nil
}

// ListTrash lists what the trash holds, newest first. An entry that cannot be
// read is left out: it can still be removed with EmptyTrash.
func (w *Workspace) ListTrash() ([]TrashedItem, error) {
	entries, err := os.ReadDir(w.trashDir())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading the trash: %w", err)
	}
	var out []TrashedItem
	for _, e := range entries {
		if item, ok := w.readTrashed(e.Name()); ok {
			out = append(out, item)
		}
	}
	slices.SortFunc(out, func(a, b TrashedItem) int { return b.TrashedAt.Compare(a.TrashedAt) })
	return out, nil
}

func (w *Workspace) readTrashed(id string) (TrashedItem, bool) {
	if !names.Valid(id) {
		return TrashedItem{}, false
	}
	data, err := os.ReadFile(filepath.Join(w.trashDir(), id, "item.json"))
	if err != nil {
		return TrashedItem{}, false
	}
	var item TrashedItem
	if json.Unmarshal(data, &item) != nil || item.ID != id || !trashedItemSafe(item) {
		return TrashedItem{}, false
	}
	return item, true
}

// trashedItemSafe reports whether item.json names only places RestoreFromTrash
// may write: a known kind, and files that sit under the item's own path inside
// requests/ or flows/, each segment a plain name. item.json is plain data on
// disk, so a damaged or edited one must not steer a restore outside Scratch.
func trashedItemSafe(item TrashedItem) bool {
	tree, ok := trashTree(item.Kind)
	if !ok || !validRelPath(item.Path) {
		return false
	}
	prefix := tree + "/" + item.Path
	for _, rel := range item.Files {
		if !validRelPath(rel) || !strings.HasPrefix(rel, prefix) {
			return false
		}
	}
	return true
}

// trashTree is the collection directory a trashed kind came from.
func trashTree(kind string) (string, bool) {
	tree, ok := map[string]string{TrashRequest: "requests", TrashFlow: "flows"}[kind]
	return tree, ok
}

// validRelPath reports whether every "/"-separated segment of p is a plain name.
func validRelPath(p string) bool {
	return p != "" && !slices.ContainsFunc(strings.Split(p, "/"), func(seg string) bool { return !names.Valid(seg) })
}

// RestoreFromTrash puts a trashed request or flow back into Scratch, under its
// old name or, when that is taken, the next free "<name>-restored". It returns
// the path it got.
func (w *Workspace) RestoreFromTrash(id string) (string, error) {
	item, ok := w.readTrashed(id)
	if !ok {
		return "", fmt.Errorf("nothing in the trash is called %q", id)
	}
	w.draftsMu.Lock()
	defer w.draftsMu.Unlock()
	m, err := w.scratch()
	if err != nil {
		return "", fmt.Errorf("opening Scratch: %w", err)
	}
	w.mu.RLock()
	c, ok := w.collection(m.ID)
	w.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("the Scratch draft is not loaded")
	}
	tree, _ := trashTree(item.Kind) // readTrashed has vetted the kind
	path := freeRestoredPath(c.Root, tree, item.Path, item.Kind)
	for _, rel := range item.Files {
		from := filepath.Join(w.trashDir(), id, "files", filepath.FromSlash(rel))
		to := filepath.Join(c.Root, filepath.FromSlash(strings.Replace(rel, tree+"/"+item.Path, tree+"/"+path, 1)))
		if err := os.MkdirAll(filepath.Dir(to), 0o750); err != nil {
			return "", fmt.Errorf("restoring %s: %w", rel, err)
		}
		if err := os.Rename(from, to); err != nil {
			return "", fmt.Errorf("restoring %s: %w", rel, err)
		}
	}
	if err := os.RemoveAll(filepath.Join(w.trashDir(), id)); err != nil {
		return path, fmt.Errorf("clearing the restored item from the trash: %w", err)
	}
	_, err = w.Reload(Change{Root: c.Root, File: c.Root, Op: store.ChangeRescan})
	return path, err
}

// freeRestoredPath is path, or path-restored, path-restored-2 … the first not taken in tree.
func freeRestoredPath(root, tree, path, kind string) string {
	suffix := map[string]string{TrashRequest: ".req.yaml", TrashFlow: flowSuffix}[kind]
	taken := func(p string) bool {
		_, err := os.Stat(filepath.Join(root, tree, filepath.FromSlash(p)+suffix))
		return err == nil
	}
	candidate := path
	for n := 1; taken(candidate); n++ {
		candidate = path + "-restored"
		if n > 1 {
			candidate = fmt.Sprintf("%s-restored-%d", path, n)
		}
	}
	return candidate
}

// DeleteFromTrash removes one item for good.
func (w *Workspace) DeleteFromTrash(id string) error {
	if !names.Valid(id) {
		return fmt.Errorf("%q is not an item of the trash", id)
	}
	if err := os.RemoveAll(filepath.Join(w.trashDir(), id)); err != nil {
		return fmt.Errorf("removing %s from the trash: %w", id, err)
	}
	return nil
}

// EmptyTrash removes everything in the trash for good.
func (w *Workspace) EmptyTrash() error {
	if err := os.RemoveAll(w.trashDir()); err != nil {
		return fmt.Errorf("emptying the trash: %w", err)
	}
	return nil
}
