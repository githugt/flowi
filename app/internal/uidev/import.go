// SPDX-License-Identifier: AGPL-3.0-or-later

package uidev

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/settings"
)

// Imports in the UI-development server run the real importers, over a collection of their own in a temporary folder,
// so the import screen shows what an import really plans (names, paths, what exists) and not a canned plan. What an
// import writes is copied into the made-up collection, so it shows in the sidebar.
//
// Before: the fake answered every preview with the same canned plan, whatever was pasted, so the import screen could not
// be seen working on a document of its own.
type importSandbox struct {
	once sync.Once
	dir  string
	ws   *flowi.Workspace
	id   model.ID
	err  error
}

// open makes the workspace and its collection on first use.
func (s *importSandbox) open() (*flowi.Workspace, model.ID, error) {
	s.once.Do(func() {
		sweepSandboxes(os.TempDir(), staleSandboxAge, time.Now())
		dir, err := os.MkdirTemp("", sandboxPrefix)
		if err != nil {
			s.err = fmt.Errorf("making the import sandbox: %w", err)
			return
		}
		s.dir = dir
		ws, _, err := flowi.OpenWorkspace(context.Background(), flowi.WorkspaceOptions{Paths: settings.Paths{ConfigDir: filepath.Join(dir, "config")}, Home: dir})
		if err != nil {
			s.err = fmt.Errorf("opening the import sandbox: %w", err)
			return
		}
		m, err := ws.InitCollection(filepath.Join(dir, "demo"), "demo", true)
		if err != nil {
			s.err = fmt.Errorf("making the import sandbox collection: %w", err)
			return
		}
		s.ws, s.id = ws, m.ID
	})
	return s.ws, s.id, s.err
}

const (
	sandboxPrefix = "flowi-uidev-import-"
	// staleSandboxAge is how old a sandbox folder must be before a later start removes it: one of a server still
	// running is younger (a server killed without cleaning up is how an old one is left).
	staleSandboxAge = 12 * time.Hour
)

// Close removes the sandbox folder, when one was made. The server calls it when it stops.
//
// Before: each run that imported anything left its folder in the temp directory for good.
func (b *Backend) Close() error {
	if b.sandbox.dir == "" {
		return nil
	}
	return os.RemoveAll(b.sandbox.dir)
}

// sweepSandboxes removes the sandbox folders of earlier runs in dir that are older than olderThan, and returns how
// many it removed. Only real folders with this server's prefix go.
func sweepSandboxes(dir string, olderThan time.Duration, now time.Time) int {
	found, err := filepath.Glob(filepath.Join(dir, sandboxPrefix+"*"))
	if err != nil {
		return 0
	}
	removed := 0
	for _, f := range found {
		info, err := os.Lstat(f)
		if err != nil || !info.IsDir() || now.Sub(info.ModTime()) < olderThan {
			continue
		}
		if os.RemoveAll(f) == nil {
			removed++
		}
	}
	return removed
}

// PreviewImportSource plans the import with the real importers into the sandbox collection.
func (b *Backend) PreviewImportSource(src flowi.ImportSource, collection model.ID) (flowi.ImportPlan, error) {
	ws, id, err := b.sandbox.open()
	if err != nil {
		return flowi.ImportPlan{}, err
	}
	plan, err := ws.PreviewImportSource(src, id)
	plan.Collection = collection // the screen knows the made-up collection by its own id
	return plan, err
}

// ApplyImportReport writes the plan into the sandbox and shows the new requests and flows in the made-up collection.
func (b *Backend) ApplyImportReport(p flowi.ImportPlan, o flowi.ApplyOptions) (flowi.ImportReport, error) {
	ws, id, err := b.sandbox.open()
	if err != nil {
		return flowi.ImportReport{}, err
	}
	into := p.Collection
	p.Collection = id
	report, err := ws.ApplyImportReport(p, o)
	if err != nil {
		return report, err
	}
	b.mirror(ws, id, into)
	return report, nil
}

// mirror copies the sandbox collection's requests and flows into the made-up collection into.
func (b *Backend) mirror(ws *flowi.Workspace, from, into model.ID) {
	var sandbox model.Collection
	for _, c := range ws.Collections() {
		if c.Manifest.ID == from {
			sandbox = c
		}
	}
	b.Mu.Lock()
	defer b.Mu.Unlock()
	for ci := range b.Cols {
		if b.Cols[ci].Manifest.ID != into {
			continue
		}
		root := b.Cols[ci].Root
		for _, r := range sandbox.Requests {
			r.File = root + "/requests/" + r.Path + ".req.yaml"
			put(&b.Cols[ci].Requests, r, func(x model.Request) string { return x.Path }, r.Path)
		}
		for _, f := range sandbox.Flows {
			f.File = root + "/flows/" + f.Path + ".arazzo.yaml"
			put(&b.Cols[ci].Flows, f, func(x model.Flow) string { return x.Path }, f.Path)
		}
		b.Cols[ci].Folders = foldersOf(b.Cols[ci].Folders, root, sandbox.Requests)
	}
}

// put replaces the item with the same path in list, or adds it. The list is cloned first: the collection was copied
// by value, so its slices are shared with whoever else holds the old collection.
func put[T any](list *[]T, item T, pathOf func(T) string, path string) {
	out := slices.Clone(*list)
	if i := slices.IndexFunc(out, func(x T) bool { return pathOf(x) == path }); i >= 0 {
		out[i] = item
	} else {
		out = append(out, item)
	}
	*list = out
}

// foldersOf adds the folders of the requests' paths that the collection does not list yet.
func foldersOf(have []model.Folder, root string, reqs []model.Request) []model.Folder {
	out := slices.Clone(have)
	for _, r := range reqs {
		parts := strings.Split(r.Path, "/")
		for i := 1; i < len(parts); i++ {
			dir := "requests/" + strings.Join(parts[:i], "/")
			if !slices.ContainsFunc(out, func(f model.Folder) bool { return f.Dir == dir }) {
				out = append(out, model.Folder{Dir: dir, File: root + "/" + dir + "/_folder.yaml"})
			}
		}
	}
	return out
}
