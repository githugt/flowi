// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ChangeOp says what happened to a file.
type ChangeOp int

// Change operations.
const (
	ChangeCreated ChangeOp = iota + 1
	ChangeModified
	ChangeRemoved
	// ChangeRescan means events were lost: reload the whole collection at
	// Root (File equals Root).
	ChangeRescan
)

// Change reports that a collection file changed. When Err is set the
// watcher has stopped because of it and the channel closes next.
type Change struct {
	Root string
	File string
	Op   ChangeOp
	Err  error
}

// coalesceWindow merges bursts of events (editors often save through a
// temporary file and a rename) into one change per file.
const coalesceWindow = 100 * time.Millisecond

// Watch reports changes to collection files under roots. Roots must be
// absolute with symlinks resolved, as Load returns them.
//
// Goroutine ownership: Watch starts one goroutine that owns the fsnotify
// watcher and the returned channel. It exits, closing the channel, when ctx
// is done or the watcher fails (after sending a Change with Err set).
func Watch(ctx context.Context, roots []string) (<-chan Change, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("starting file watcher: %w", err)
	}
	dirs := map[string]bool{}
	for _, root := range roots {
		if err := addTree(w, root, dirs); err != nil {
			_ = w.Close() // already returning the more useful error
			return nil, err
		}
	}
	out := make(chan Change)
	go runWatcher(ctx, w, roots, dirs, out)
	return out, nil
}

// addTree watches root and every directory below it, recording each in dirs.
func addTree(w *fsnotify.Watcher, root string, dirs map[string]bool) error {
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if d.Name() == ".git" || strings.HasSuffix(d.Name(), examplesDir) {
			return filepath.SkipDir
		}
		dirs[p] = true
		return w.Add(p)
	})
	if err != nil {
		return fmt.Errorf("watching %s: %w", root, err)
	}
	return nil
}

// pendingChange accumulates events for one file until the window passes.
type pendingChange struct {
	root    string
	created bool // every event so far was a create
	rescan  bool
}

// runWatcher is the watcher goroutine. dirs is owned by it alone.
func runWatcher(ctx context.Context, w *fsnotify.Watcher, roots []string, dirs map[string]bool, out chan<- Change) {
	defer close(out)
	defer w.Close() // closing a watcher we own; its error tells us nothing actionable
	pending := map[string]pendingChange{}
	timer := time.NewTimer(coalesceWindow)
	timer.Stop()
	// The timer starts at the first pending event and is not pushed back by
	// later ones, so a file written continuously cannot starve the others.
	armed := false
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			if handleEvent(w, roots, dirs, ev, pending) && !armed {
				timer.Reset(coalesceWindow)
				armed = true
			}
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			if errors.Is(err, fsnotify.ErrEventOverflow) {
				// Events were lost; ask consumers to reload everything.
				for _, root := range roots {
					pending[root] = pendingChange{root: root, rescan: true}
				}
				if !armed {
					timer.Reset(coalesceWindow)
					armed = true
				}
				continue
			}
			send(ctx, out, Change{Err: fmt.Errorf("file watcher: %w", err)})
			return
		case <-timer.C:
			armed = false
			if !flush(ctx, pending, out) {
				return
			}
		}
	}
}

// handleEvent records an event for a relevant file and watches new
// directories. It reports whether something was recorded.
func handleEvent(w *fsnotify.Watcher, roots []string, dirs map[string]bool, ev fsnotify.Event, pending map[string]pendingChange) bool {
	root, rel, ok := relativeTo(roots, ev.Name)
	if !ok {
		return false
	}
	if ev.Has(fsnotify.Create) {
		if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
			return recordNewDirectory(w, root, ev.Name, dirs, pending)
		}
	}
	if (ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename)) && dirs[ev.Name] {
		// A vanished directory: consumers drop everything beneath it.
		forgetTree(dirs, ev.Name)
		pending[ev.Name] = pendingChange{root: root}
		return true
	}
	if rel != manifestName && classify(rel) == kindIgnored {
		return false
	}
	record(pending, root, ev.Name, ev.Has(fsnotify.Create))
	return true
}

func record(pending map[string]pendingChange, root, file string, created bool) {
	p, seen := pending[file]
	pending[file] = pendingChange{root: root, created: created && (!seen || p.created)}
}

// recordNewDirectory watches a directory that appeared (created or moved
// in) and records every collection file already inside it: files that
// arrive with the directory produce no events of their own.
func recordNewDirectory(w *fsnotify.Watcher, root, dir string, dirs map[string]bool, pending map[string]pendingChange) bool {
	_ = addTree(w, dir, dirs) // best effort: a directory removed again at once is not an error
	found := false
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil // unreadable parts are reported when the collection reloads
		}
		if rel, relErr := filepath.Rel(root, p); relErr == nil && classify(filepath.ToSlash(rel)) != kindIgnored {
			record(pending, root, p, true)
			found = true
		}
		return nil
	})
	return found
}

// forgetTree removes dir and everything below it from the watched set.
func forgetTree(dirs map[string]bool, dir string) {
	prefix := dir + string(filepath.Separator)
	for d := range dirs {
		if d == dir || strings.HasPrefix(d, prefix) {
			delete(dirs, d)
		}
	}
}

func relativeTo(roots []string, file string) (string, string, bool) {
	for _, root := range roots {
		rel, err := filepath.Rel(root, file)
		if err == nil && !strings.HasPrefix(rel, "..") {
			return root, filepath.ToSlash(rel), true
		}
	}
	return "", "", false
}

// flush emits one change per pending file. The final state on disk decides
// the operation, which is what makes rename-saves look like modifications.
func flush(ctx context.Context, pending map[string]pendingChange, out chan<- Change) bool {
	for file, p := range pending {
		op := ChangeModified
		if p.rescan {
			op = ChangeRescan
		} else if _, err := os.Stat(file); errors.Is(err, fs.ErrNotExist) {
			op = ChangeRemoved
		} else if p.created {
			op = ChangeCreated
		}
		if !send(ctx, out, Change{Root: p.root, File: file, Op: op}) {
			return false
		}
		delete(pending, file)
	}
	return true
}

func send(ctx context.Context, out chan<- Change, c Change) bool {
	select {
	case out <- c:
		return true
	case <-ctx.Done():
		return false
	}
}
