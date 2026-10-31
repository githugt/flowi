// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/workspace"
)

// A draft is a collection kept in Flowi's own directory
// (workspace.DraftsDir) until the user saves it to a folder of their own.
// It is an ordinary mounted collection: being a draft is read from where
// it is mounted, so workspace.yaml has no draft flag to keep in step.

const (
	scratchDir  = "scratch"
	scratchName = "Scratch"
	// untitledLimit bounds the search for a free "untitled-N" name.
	untitledLimit = 10000
)

// ErrNotDraft is returned when a draft-only operation is asked of a
// collection that lives in the user's own folder.
var ErrNotDraft = errors.New("the collection is not a draft")

func (w *Workspace) draftsDir() string { return workspace.DraftsDir(w.opts.Paths, w.opts.Name) }

func (w *Workspace) legacyDraftsDir() string {
	return workspace.LegacyDraftsDir(w.opts.Paths, w.opts.Name)
}

// NewDraft creates and mounts an empty draft collection.
func (w *Workspace) NewDraft(name string) (model.Manifest, error) {
	id := w.newID()
	m, err := w.initCollection(filepath.Join(w.draftsDir(), string(id)), name, id, true)
	if err != nil {
		return model.Manifest{}, fmt.Errorf("creating draft %s: %w", name, err)
	}
	return m, nil
}

// Scratch returns the "Scratch" draft that holds requests made outside any
// collection, creating or re-mounting it as needed.
func (w *Workspace) Scratch() (model.Manifest, error) {
	w.draftsMu.Lock()
	defer w.draftsMu.Unlock()
	return w.scratch()
}

func (w *Workspace) scratch() (model.Manifest, error) {
	// A Scratch made before drafts belonged to a user is kept, not replaced.
	for _, base := range []string{w.draftsDir(), w.legacyDraftsDir()} {
		dir := filepath.Join(base, scratchDir)
		if c, ok := w.mountedAt(dir); ok {
			return c.Manifest, nil
		}
		if _, err := os.Stat(filepath.Join(dir, "flowi.yaml")); err == nil {
			return w.Mount(dir)
		}
	}
	return w.InitCollection(filepath.Join(w.draftsDir(), scratchDir), scratchName, true)
}

// NewScratchRequest creates the next free "untitled" request in Scratch.
func (w *Workspace) NewScratchRequest() (RequestTarget, error) {
	w.draftsMu.Lock()
	defer w.draftsMu.Unlock()
	m, err := w.scratch()
	if err != nil {
		return RequestTarget{}, fmt.Errorf("opening Scratch: %w", err)
	}
	w.mu.RLock()
	c, ok := w.collection(m.ID)
	w.mu.RUnlock()
	if !ok {
		return RequestTarget{}, fmt.Errorf("scratch collection %s is not loaded", m.ID)
	}
	path, err := freeUntitled(c)
	if err != nil {
		return RequestTarget{}, err
	}
	return w.NewRequest(m.ID, path)
}

// freeUntitled picks untitled, untitled-2, … not taken by a request file.
func freeUntitled(c model.Collection) (string, error) {
	for n := 1; n <= untitledLimit; n++ {
		path := "untitled"
		if n > 1 {
			path = fmt.Sprintf("untitled-%d", n)
		}
		file := filepath.Join(c.Root, "requests", path+".req.yaml")
		if _, err := os.Stat(file); errors.Is(err, fs.ErrNotExist) {
			return path, nil
		}
	}
	return "", fmt.Errorf("no free untitled request name in %s", c.Manifest.Name)
}

// IsDraft reports whether the mounted collection id is a draft.
func (w *Workspace) IsDraft(id model.ID) bool {
	_, ok := w.draftRoot(id)
	return ok
}

// SaveDraft moves draft id into dir, which must be missing or empty and
// not inside the draft. The collection keeps its id, so its history,
// cookies and secrets follow it. If copying fails, what was copied is
// removed and the draft stays as it was.
func (w *Workspace) SaveDraft(id model.ID, dir string) error {
	w.draftsMu.Lock()
	defer w.draftsMu.Unlock()
	root, ok := w.draftRoot(id)
	if !ok {
		return ErrNotDraft
	}
	target, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", dir, err)
	}
	existed, err := checkSaveTarget(workspace.Dir(w.opts.Paths, w.opts.Name), target)
	if err != nil {
		return err
	}
	if err := copyDraft(root, target); err != nil {
		removeCopy(target, existed)
		return fmt.Errorf("copying the draft to %s: %w", target, err)
	}
	if err := w.Relocate(id, target); err != nil {
		removeCopy(target, existed)
		return err
	}
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("saved to %s, but the draft copy in %s could not be removed: %w", target, root, err)
	}
	return nil
}

// DiscardDraft unmounts draft id and deletes its files and its data
// (history, saved responses, cookies). User folders are never touched.
func (w *Workspace) DiscardDraft(id model.ID) error {
	w.draftsMu.Lock()
	defer w.draftsMu.Unlock()
	root, ok := w.draftRoot(id)
	if !ok {
		return ErrNotDraft
	}
	if err := w.Unmount(id); err != nil {
		return err
	}
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("removing draft %s: %w", root, err)
	}
	if err := os.RemoveAll(w.DataDir(id)); err != nil {
		return fmt.Errorf("removing draft data: %w", err)
	}
	return nil
}

// draftRoot is the directory of mounted collection id when it lies under
// the drafts directory.
func (w *Workspace) draftRoot(id model.ID) (string, bool) {
	w.mu.RLock()
	c, ok := w.collection(id)
	w.mu.RUnlock()
	if !ok {
		return "", false
	}
	return c.Root, insideDir(w.draftsDir(), c.Root) || insideDir(w.legacyDraftsDir(), c.Root)
}

func (w *Workspace) mountedAt(dir string) (model.Collection, bool) {
	want := resolvedPath(dir)
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, c := range w.collections {
		if resolvedPath(c.Root) == want {
			return c, true
		}
	}
	return model.Collection{}, false
}

// resolvedPath resolves symlinks (macOS temp and config dirs often sit
// behind one) so paths compare by where they really are. A path that does
// not exist yet is resolved through its nearest existing ancestor.
func resolvedPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	rest := ""
	for dir := abs; ; dir = filepath.Dir(dir) {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(resolved, rest)
		}
		if filepath.Dir(dir) == dir {
			return abs
		}
		rest = filepath.Join(filepath.Base(dir), rest)
	}
}

// insideDir reports whether path is strictly inside parent.
func insideDir(parent, path string) bool {
	rel, err := filepath.Rel(resolvedPath(parent), resolvedPath(path))
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// checkSaveTarget refuses a target inside Flowi's own workspace directory
// (drafts and their data live there, and discarding a draft deletes its
// folder) or one that holds anything; it reports whether the (empty)
// target already existed.
func checkSaveTarget(wsDir, target string) (bool, error) {
	if resolvedPath(target) == resolvedPath(wsDir) || insideDir(wsDir, target) {
		return false, fmt.Errorf("%s is inside Flowi's own folder; choose a folder of your own", target)
	}
	entries, err := os.ReadDir(target)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking %s: %w", target, err)
	}
	if len(entries) > 0 {
		return false, fmt.Errorf("%s is not empty; choose an empty or new folder", target)
	}
	return true, nil
}

// copyDraft copies directories and regular files from root to target.
// Drafts are written only by Flowi, so anything else (a symlink, a
// device) is left out rather than followed.
func copyDraft(root, target string) error {
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		dst := filepath.Join(target, rel)
		switch {
		case d.IsDir():
			// Collection directories live in user repos and are normally world-readable.
			return os.MkdirAll(dst, 0o755) //nolint:gosec // see above
		case d.Type().IsRegular():
			return copyFile(p, dst)
		default:
			return nil
		}
	})
	return err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src) //nolint:gosec // src is inside Flowi's own drafts directory
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()                                      // read-only: a close error loses nothing
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644) //nolint:gosec // collection files are normally world-readable
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close() // the copy error is the one to report
		return err
	}
	return out.Close()
}

// removeCopy undoes a failed save. target was missing or empty before the
// copy began, so everything in it is ours to remove; a folder the user
// picked is kept, emptied.
//
// Best effort: the caller is already returning the error that matters.
func removeCopy(target string, existed bool) {
	if !existed {
		_ = os.RemoveAll(target)
		return
	}
	entries, _ := os.ReadDir(target)
	for _, e := range entries {
		_ = os.RemoveAll(filepath.Join(target, e.Name()))
	}
}

// SessionDir is where the GUI keeps what carries over between sessions of
// the default user (open tabs, and the run log and history when kept). The
// library does not read or write there; it only names the place.
func (w *Workspace) SessionDir() string {
	return filepath.Join(workspace.UserDir(w.opts.Paths, w.opts.Name, workspace.DefaultUser), "session")
}
