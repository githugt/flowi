// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/model"
)

// DepRow is a dependency of a collection and whether it is mounted.
type DepRow struct {
	Name, Repo string
	Mounted    bool
}

// CollectionRow is one mounted collection in the workspace manager.
type CollectionRow struct {
	ID      model.ID
	Name    string
	Path    string // as written in workspace.yaml
	Missing bool   // its directory is gone: Relocate it
	Draft   bool   // kept in Flow-I's own directory: SaveDraft or Discard it
	Deps    []DepRow
}

// Manager is the workspace manager (design §10.4): mount, unmount, locate
// missing collections, create new ones, and see dependency status.
type Manager struct {
	b        Backend
	w        *Worker
	ws       *Workspace
	rows     []CollectionRow
	err      string
	onChange []func()
}

// NewManager returns the manager; ws is reloaded after every action.
func NewManager(b Backend, w *Worker, ws *Workspace) *Manager {
	return &Manager{b: b, w: w, ws: ws}
}

// OnChange registers f to run after the rows or the error change.
func (m *Manager) OnChange(f func()) { m.onChange = append(m.onChange, f) }

func (m *Manager) changed() {
	for _, f := range m.onChange {
		f()
	}
}

// Rows are the mounted collections, loaded first, then missing ones.
func (m *Manager) Rows() []CollectionRow { return m.rows }

// Err is the last action's problem, or "".
func (m *Manager) Err() string { return m.err }

// Load reads the workspace in the background.
func (m *Manager) Load() {
	Run(m.w, func() []CollectionRow { return managerRows(m.b) }, func(rows []CollectionRow) {
		m.rows = rows
		m.changed()
	})
}

func managerRows(b Backend) []CollectionRow {
	cs, paths := b.Collections(), b.MountedPaths()
	loaded := map[model.ID]bool{}
	for _, c := range cs {
		loaded[c.Manifest.ID] = true
	}
	var rows []CollectionRow
	for _, c := range cs {
		row := CollectionRow{ID: c.Manifest.ID, Name: c.Manifest.Name, Path: paths[c.Manifest.ID], Draft: b.IsDraft(c.Manifest.ID)}
		for _, d := range c.Manifest.DependsOn {
			row.Deps = append(row.Deps, DepRow{Name: d.Name, Repo: d.Repo, Mounted: loaded[d.ID]})
		}
		rows = append(rows, row)
	}
	for _, id := range b.Missing() {
		rows = append(rows, CollectionRow{ID: id, Path: paths[id], Missing: true})
	}
	slices.SortStableFunc(rows[:len(cs)], func(a, b CollectionRow) int { return strings.Compare(a.Name, b.Name) })
	return rows
}

// Mount mounts the collection in dir.
func (m *Manager) Mount(dir string) {
	m.act(func() error { _, err := m.b.Mount(dir); return err })
}

// Unmount removes a collection from the workspace (its files stay).
func (m *Manager) Unmount(id model.ID) { m.act(func() error { return m.b.Unmount(id) }) }

// Relocate points a missing collection at its new directory.
func (m *Manager) Relocate(id model.ID, dir string) {
	m.act(func() error { return m.b.Relocate(id, dir) })
}

// Init creates a collection named name in dir and mounts it.
func (m *Manager) Init(dir, name string) {
	m.act(func() error { _, err := m.b.InitCollection(dir, name); return err })
}

// NewCollection creates a collection named name: in location when given
// (the folder is created or must be empty), else in Flow-I's own folder.
func (m *Manager) NewCollection(name, location string) {
	if location == "" {
		m.NewDraft(name)
		return
	}
	m.Init(location, name)
}

// NewDraft creates a collection named name in Flow-I's own folder.
func (m *Manager) NewDraft(name string) {
	m.act(func() error { _, err := m.b.NewDraft(name); return err })
}

// SaveDraft moves draft id into dir (missing or empty).
func (m *Manager) SaveDraft(id model.ID, dir string) {
	m.act(func() error { return m.b.SaveDraft(id, dir) })
}

// Discard deletes draft id and its files.
func (m *Manager) Discard(id model.ID) { m.act(func() error { return m.b.DiscardDraft(id) }) }

// SaveDraftOr is SaveDraft for callers outside the manager screen: fail gets
// the error, which the screen (usually closed) would otherwise keep to itself.
func (m *Manager) SaveDraftOr(id model.ID, dir string, fail func(error)) {
	m.actOr(func() error { return m.b.SaveDraft(id, dir) }, fail)
}

// DiscardOr is Discard with the error handed to fail.
func (m *Manager) DiscardOr(id model.ID, fail func(error)) {
	m.actOr(func() error { return m.b.DiscardDraft(id) }, fail)
}

// act runs an action, then reloads the manager and the workspace tree.
func (m *Manager) act(action func() error) { m.actOr(action, nil) }

func (m *Manager) actOr(action func() error, fail func(error)) {
	Run(m.w, action, func(err error) {
		m.err = ""
		if err != nil {
			m.err = ErrorText(err)
			if fail != nil {
				fail(err)
			}
		}
		m.changed()
		m.Load()
		m.ws.Load()
		m.ws.Rewatch() // the mounted roots may have changed
	})
}
