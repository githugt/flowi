// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
)

// DepView is a dependency of a collection.
type DepView struct {
	Name    string `json:"name"`
	Repo    string `json:"repo"`
	Mounted bool   `json:"mounted"`
}

// CollectionView is a mounted collection in the manager.
type CollectionView struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Missing bool      `json:"missing"` // its directory is gone: relocate it
	Draft   bool      `json:"draft"`   // kept in Flowi's own folder: save it somewhere or discard it
	Deps    []DepView `json:"deps"`
}

// ManagerView is the workspace manager.
type ManagerView struct {
	Collections []CollectionView `json:"collections"`
	Error       string           `json:"error,omitempty"`
}

// ManagerService mounts, unmounts, relocates and creates collections.
type ManagerService struct {
	app   *App
	vm    *vm.Manager
	first *firstLoad
	// pending is the last Update from spec previewed; used on the actor only.
	pending *pendingSpecUpdate
}

// newManagerService builds the service for the workspace manager.
func newManagerService(a *App) *ManagerService {
	s := &ManagerService{app: a, vm: vm.NewManager(a.backend, a.worker, a.workspace), first: newFirstLoad()}
	s.vm.OnChange(func() {
		s.first.mark()
		a.emit("manager", s.view())
	})
	return s
}

// State loads the collections and returns them; changes arrive as the "manager" event.
func (s *ManagerService) State() ManagerView {
	var v ManagerView
	s.app.actor.Do(s.vm.Load)
	s.first.wait()
	s.app.actor.Do(func() { v = s.view() })
	return v
}

// Mount mounts the collection in dir.
func (s *ManagerService) Mount(dir string) { s.app.actor.Do(func() { s.vm.Mount(dir) }) }

// OpenArazzo opens an Arazzo document (a file or an http(s) address) as a collection whose flows run against the
// server the document's OpenAPI description names, until the app quits.
func (s *ManagerService) OpenArazzo(ref string) { s.app.actor.Do(func() { s.vm.OpenArazzo(ref) }) }

// OpenBundle opens a run bundle file as a collection whose runs play the recording back.
func (s *ManagerService) OpenBundle(path string) { s.app.actor.Do(func() { s.vm.OpenBundle(path) }) }

// Unmount removes a collection from the workspace; its files stay.
func (s *ManagerService) Unmount(id string) { s.app.actor.Do(func() { s.vm.Unmount(model.ID(id)) }) }

// Relocate points a missing collection at its new directory.
func (s *ManagerService) Relocate(id, dir string) {
	s.app.actor.Do(func() { s.vm.Relocate(model.ID(id), dir) })
}

// NewCollection creates a collection: in location when given (the folder
// is created or must be empty), else in Flowi's own folder.
func (s *ManagerService) NewCollection(name, location string) {
	s.app.actor.Do(func() { s.vm.NewCollection(name, location) })
}

// SaveDraft moves a draft collection into dir.
func (s *ManagerService) SaveDraft(id, dir string) {
	s.app.actor.Do(func() { s.vm.SaveDraft(model.ID(id), dir) })
}

// Discard deletes a draft collection and its files.
func (s *ManagerService) Discard(id string) { s.app.actor.Do(func() { s.vm.Discard(model.ID(id)) }) }

func (s *ManagerService) view() ManagerView {
	v := ManagerView{Collections: []CollectionView{}, Error: s.vm.Err()}
	for _, r := range s.vm.Rows() {
		c := CollectionView{ID: string(r.ID), Name: r.Name, Path: r.Path, Missing: r.Missing, Draft: r.Draft, Deps: []DepView{}}
		for _, d := range r.Deps {
			c.Deps = append(c.Deps, DepView{Name: d.Name, Repo: d.Repo, Mounted: d.Mounted})
		}
		v.Collections = append(v.Collections, c)
	}
	return v
}
