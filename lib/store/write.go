// SPDX-License-Identifier: Apache-2.0

package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/githugt/flowi-lib/arazzo"
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

const newFileMode fs.FileMode = 0o644

// Apply writes each change atomically: the new content goes to a temporary
// file in the same directory, which then replaces the target. Existing
// files keep their permissions; new files get 0644.
func Apply(changes []FileChange) error {
	for _, ch := range changes {
		if err := writeAtomic(ch.File, ch.New); err != nil {
			return err
		}
	}
	return nil
}

func writeAtomic(file string, data []byte) error {
	mode := newFileMode
	if info, err := os.Stat(file); err == nil {
		mode = info.Mode().Perm()
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("checking %s: %w", file, err)
	}
	dir := filepath.Dir(file)
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // collection directories live in user repos and are normally world-readable
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".flowi-*")
	if err != nil {
		return fmt.Errorf("creating temporary file for %s: %w", file, err)
	}
	// Removing the temp file after a successful rename fails harmlessly.
	defer os.Remove(tmp.Name())
	if err := writeAndClose(tmp, data, mode); err != nil {
		return fmt.Errorf("writing %s: %w", file, err)
	}
	if err := os.Rename(tmp.Name(), file); err != nil {
		return fmt.Errorf("replacing %s: %w", file, err)
	}
	return nil
}

func writeAndClose(f *os.File, data []byte, mode fs.FileMode) error {
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return err
	}
	return os.Chmod(f.Name(), mode)
}

func fileBase(file string) string { return filepath.Base(file) }

// EncodeRequest is the canonical file content of r.
func (s *Store) EncodeRequest(r model.Request) ([]byte, error) { return s.codec.EncodeRequest(r) }

// EncodeEnvironment is the canonical file content of e. A name equal to
// the file stem is left out, as Load derives it.
func (s *Store) EncodeEnvironment(e model.Environment) ([]byte, error) {
	return s.codec.EncodeEnvironment(withoutDefaultName(e))
}

// HasComments reports whether file content has comments, which the
// canonical encoding does not keep.
func (s *Store) HasComments(data []byte) bool { return s.codec.HasComments(data) }

// EncodeFlow is the canonical file content of f: an Arazzo document whose
// steps name c's requests by operationId (see EncodeFlow in package arazzo).
func (s *Store) EncodeFlow(c model.Collection, f model.Flow) ([]byte, error) {
	data, _, err := arazzo.EncodeFlow(f, s.flowOptions(c, f.Path))
	return data, err
}

// EncodeFlowBody is the flow's own data in canonical form: the text kept in
// the Arazzo file's x-flowi-flow, and what the editor's code view shows.
func (s *Store) EncodeFlowBody(f model.Flow) ([]byte, error) { return s.codec.EncodeFlow(f) }

// DecodeFlowBody reads the text EncodeFlowBody wrote; diagnostics are
// positioned in data.
func (s *Store) DecodeFlowBody(file, path string, data []byte) (model.Flow, diag.List) {
	return s.codec.DecodeFlow(file, path, data)
}

// EncodeBlueprint is the canonical file content of b.
func (s *Store) EncodeBlueprint(b model.Blueprint) ([]byte, error) { return s.codec.EncodeBlueprint(b) }

// EncodeFolder is the canonical _folder.yaml for f.
func (s *Store) EncodeFolder(f model.Folder) ([]byte, error) { return s.codec.EncodeFolder(f) }
