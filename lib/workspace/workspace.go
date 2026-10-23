// SPDX-License-Identifier: Apache-2.0

// Package workspace manages the user's workspace: workspace.yaml, the
// collections mounted into it, and its private data directory. A
// workspace is never stored in a collection repository.
package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/ids"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/settings"
	"github.com/githugt/flowi-lib/store"
)

// DefaultName is the workspace used when none is named.
const DefaultName = "default"

const fileName = "workspace.yaml"

// Codec converts workspace.yaml to and from the model.
type Codec interface {
	DecodeWorkspace(file string, data []byte) (model.Workspace, diag.List)
	EncodeWorkspace(model.Workspace) ([]byte, error)
}

// LoadOrCreate reads the named workspace, creating it with a fresh id on
// first use.
func LoadOrCreate(p settings.Paths, name string, c Codec, idSource *ids.Source) (model.Workspace, diag.List, error) {
	file := filepath.Join(Dir(p, name), fileName)
	data, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		ws := model.Workspace{Meta: model.Meta{ID: idSource.New(), Name: displayName(name)}, File: file}
		return ws, nil, Save(p, c, ws)
	}
	if err != nil {
		return model.Workspace{}, nil, fmt.Errorf("reading %s: %w", file, err)
	}
	ws, diags := c.DecodeWorkspace(file, data)
	ws.File = file
	return ws, diags, nil
}

func displayName(name string) string {
	if name == DefaultName {
		return "Default"
	}
	return name
}

// Save writes workspace.yaml atomically. Its location comes from ws.File,
// or the default location for the workspace's directory when unset.
func Save(p settings.Paths, c Codec, ws model.Workspace) error {
	file := ws.File
	if file == "" {
		file = filepath.Join(Dir(p, DefaultName), fileName)
	}
	data, err := c.EncodeWorkspace(ws)
	if err != nil {
		return fmt.Errorf("encoding workspace: %w", err)
	}
	return store.Apply([]store.FileChange{{File: file, New: data}})
}
