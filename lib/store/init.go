// SPDX-License-Identifier: Apache-2.0

package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/githugt/flowi-lib/model"
)

// Init creates a new collection in dir: flowi.yaml plus the requests and
// flows directories. It refuses a directory that already
// holds a collection.
func (s *Store) Init(dir string, m model.Manifest) error {
	manifest := filepath.Join(dir, manifestName)
	if _, err := os.Stat(manifest); err == nil {
		return fmt.Errorf("%s: %w", dir, ErrAlreadyCollection)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("checking %s: %w", manifest, err)
	}
	for _, sub := range []string{"requests", "flows"} {
		// Collection directories live in user repos and are normally world-readable.
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil { //nolint:gosec // see above
			return fmt.Errorf("creating %s: %w", sub, err)
		}
	}
	data, err := s.codec.EncodeManifest(m)
	if err != nil {
		return fmt.Errorf("encoding manifest: %w", err)
	}
	return Apply([]FileChange{{File: manifest, New: data}})
}
