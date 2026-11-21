// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"errors"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
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

// settle runs start on the actor and waits for the outcome it reports, as an error with a message for people.
// Before: the screen sent each of these actions and cleared its fields at once, so a failure left the person to find
// the error at the top of the page and type everything again.
func (s *ManagerService) settle(start func(done func(error))) error {
	out := make(chan error, 1)
	s.app.actor.Do(func() {
		start(func(err error) {
			if err != nil {
				err = errors.New(vm.ErrorText(err))
			}
			out <- err
		})
	})
	return <-out
}

// Mount mounts the collection in dir. A refusal comes back as the error.
func (s *ManagerService) Mount(dir string) error {
	return s.settle(func(done func(error)) { s.vm.MountThen(dir, done) })
}

// OpenArazzo opens an Arazzo document (a file or an http(s) address) as a collection whose flows run against the
// server the document's OpenAPI description names, until the app quits.
func (s *ManagerService) OpenArazzo(ref string) error {
	return s.settle(func(done func(error)) { s.vm.OpenArazzoThen(ref, done) })
}

// OpenExample opens the example collection with a demo server of its own (made in this process, on a free port, gone
// when the app quits) and a "demo" environment that points at it, so its flows run with nothing set up.
func (s *ManagerService) OpenExample() error {
	return s.settle(func(done func(error)) { s.vm.OpenExampleThen(done) })
}

// AllowSecretHost lets secrets be sent to a host the selected environment does not name: it goes on the person's own
// list (secrets.allowed_hosts), which a collection cannot change. A refusal (not a host) comes back as the error.
func (s *ManagerService) AllowSecretHost(host string) error {
	return s.settle(func(done func(error)) { s.vm.AllowSecretHostThen(host, done) })
}

// OpenBundle opens a run bundle file as a collection whose runs play the recording back.
func (s *ManagerService) OpenBundle(path string) error {
	return s.settle(func(done func(error)) { s.vm.OpenBundleThen(path, done) })
}

// RepoAdvice says what to do about the generated files of a collection kept in a git repository (empty when
// nothing needs doing, or the collection is not in a repository).
func (s *ManagerService) RepoAdvice(id string) []string {
	var dir string
	s.app.actor.Do(func() {
		for _, c := range s.view().Collections {
			if c.ID == id && !c.Missing {
				dir = c.Path
			}
		}
	})
	if dir == "" {
		return []string{}
	}
	out := vm.RepoAdvice(dir) // walks the disk: outside the actor
	if out == nil {
		out = []string{}
	}
	return out
}

// Unmount removes a collection from the workspace; its files stay.
func (s *ManagerService) Unmount(id string) { s.app.actor.Do(func() { s.vm.Unmount(model.ID(id)) }) }

// Relocate points a missing collection at its new directory.
func (s *ManagerService) Relocate(id, dir string) error {
	return s.settle(func(done func(error)) { s.vm.RelocateThen(model.ID(id), dir, done) })
}

// NewCollection creates a collection: in location when given (the folder
// is created or must be empty), else in Flowi's own folder.
func (s *ManagerService) NewCollection(name, location string) error {
	return s.settle(func(done func(error)) { s.vm.NewCollectionThen(name, location, done) })
}

// SaveDraft moves a draft collection into dir.
func (s *ManagerService) SaveDraft(id, dir string) error {
	return s.settle(func(done func(error)) { s.vm.SaveDraftThen(model.ID(id), dir, done) })
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
