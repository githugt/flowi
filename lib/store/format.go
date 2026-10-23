// SPDX-License-Identifier: Apache-2.0

package store

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/model"
)

// FileChange is a file whose canonical content differs from disk. Old is
// nil for a new file. DropsComments is set when Old has comments, which
// the canonical form does not keep.
type FileChange struct {
	File          string
	Old           []byte
	New           []byte
	DropsComments bool
}

// Format returns the files of c whose canonical encoding differs from what
// was loaded, after repairing path hints within the collection. It writes
// nothing; pass the result to Apply.
func (s *Store) Format(c model.Collection) ([]FileChange, error) {
	c = RepairRefs(c, LocalResolver(c))
	var changes []FileChange
	add := func(file string, encode func() ([]byte, error)) error {
		data, err := encode()
		if err != nil {
			return fmt.Errorf("formatting %s: %w", file, err)
		}
		old, _ := s.source(file)
		if !bytes.Equal(old, data) {
			changes = append(changes, FileChange{File: file, Old: old, New: data, DropsComments: old != nil && s.codec.HasComments(old)})
		}
		return nil
	}
	if err := add(c.Manifest.File, func() ([]byte, error) { return s.codec.EncodeManifest(c.Manifest) }); err != nil {
		return nil, err
	}
	for _, f := range c.Folders {
		if err := add(f.File, func() ([]byte, error) { return s.codec.EncodeFolder(f) }); err != nil {
			return nil, err
		}
	}
	for _, e := range c.Environments {
		if err := add(e.File, func() ([]byte, error) { return s.codec.EncodeEnvironment(withoutDefaultName(e)) }); err != nil {
			return nil, err
		}
	}
	for _, r := range c.Requests {
		if err := add(r.File, func() ([]byte, error) { return s.codec.EncodeRequest(r) }); err != nil {
			return nil, err
		}
	}
	for _, f := range c.Flows {
		if err := add(f.File, func() ([]byte, error) { return s.EncodeFlow(c, f) }); err != nil {
			return nil, err
		}
	}
	for _, b := range c.Blueprints {
		if err := add(b.File, func() ([]byte, error) { return s.codec.EncodeBlueprint(b) }); err != nil {
			return nil, err
		}
	}
	slices.SortFunc(changes, func(a, b FileChange) int { return strings.Compare(a.File, b.File) })
	return changes, nil
}

// withoutDefaultName drops a name that Load derived from the file stem, so
// formatting does not write it into the file.
func withoutDefaultName(e model.Environment) model.Environment {
	stem := strings.TrimSuffix(fileBase(e.File), ".yaml")
	if e.Name == stem {
		e.Name = ""
	}
	return e
}
