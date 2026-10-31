// SPDX-License-Identifier: Apache-2.0

// Package store loads collections from disk, resolves diagnostic positions,
// produces canonical formatting changes and writes files atomically.
package store

import (
	"crypto/rand"
	"errors"
	"sync"
	"time"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/ids"
	"github.com/githugt/flowi/lib/model"
)

// Errors returned by the store.
var (
	ErrNotCollection     = errors.New("directory is not a collection (no flowi.yaml)")
	ErrAlreadyCollection = errors.New("directory already contains flowi.yaml")
)

// Codec converts collection files to and from the model.
type Codec interface {
	DecodeManifest(file string, data []byte) (model.Manifest, diag.List)
	DecodeFolder(file, dir string, data []byte) (model.Folder, diag.List)
	DecodeEnvironment(file string, data []byte) (model.Environment, diag.List)
	DecodeRequest(file, path string, data []byte) (model.Request, diag.List)
	DecodeFlow(file, path string, data []byte) (model.Flow, diag.List)
	DecodeBlueprint(file, path string, data []byte) (model.Blueprint, diag.List)
	EncodeManifest(model.Manifest) ([]byte, error)
	EncodeFolder(model.Folder) ([]byte, error)
	EncodeEnvironment(model.Environment) ([]byte, error)
	EncodeRequest(model.Request) ([]byte, error)
	EncodeFlow(model.Flow) ([]byte, error)
	EncodeBlueprint(model.Blueprint) ([]byte, error)
	Locate(data []byte, yamlPath string) (diag.Position, bool)
	HasComments(data []byte) bool
}

// Store reads and writes collections. It remembers the bytes of every file
// it loaded so diagnostics can be positioned and formatting compared
// without re-reading the disk.
type Store struct {
	codec   Codec
	ids     *ids.Source       // ids for flows read from foreign Arazzo documents
	mu      sync.Mutex        // guards sources
	sources map[string][]byte // absolute path → bytes as last loaded
}

// New returns a store using codec.
func New(c Codec) *Store {
	return &Store{codec: c, ids: ids.NewSource(time.Now, rand.Reader), sources: map[string][]byte{}}
}

func (s *Store) remember(file string, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sources[file] = data
}

func (s *Store) forget(file string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sources, file)
}

func (s *Store) source(file string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.sources[file]
	return data, ok
}

// Locate fills in the position of a diagnostic (and its related locations)
// from its YAML path, when the position is not already known.
func (s *Store) Locate(d diag.Diagnostic) diag.Diagnostic {
	d.Location = s.locate(d.Location)
	related := make([]diag.Location, 0, len(d.Related))
	for _, r := range d.Related {
		related = append(related, s.locate(r))
	}
	if len(related) > 0 {
		d.Related = related
	}
	return d
}

func (s *Store) locate(loc diag.Location) diag.Location {
	if loc.Pos.Line > 0 || loc.Path == "" {
		return loc
	}
	data, ok := s.source(loc.File)
	if !ok {
		return loc
	}
	if pos, found := s.codec.Locate(data, flowYAMLPath(loc.File, loc.Path)); found {
		loc.Pos = pos
	}
	return loc
}
