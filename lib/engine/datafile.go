// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/internal/names"
	"github.com/githugt/flowi/lib/internal/nodes"
)

// dataDirs are the directories nodes may write to.
func dataDirs() []string { return []string{"saved", "digests"} }

// WriteFile writes a save/digest file under the flow collection's data
// directory: <dir>/<flow-id>/<name>. Every segment of name must be a plain
// file name, so collection content can never write elsewhere.
func (rt *nodeRuntime) WriteFile(dir, name string, data []byte, appendTo bool) (string, error) {
	segments := strings.Split(name, "/")
	for _, s := range append([]string{dir}, segments...) {
		if !names.Valid(s) {
			return "", fmt.Errorf("%w: %q", nodes.ErrFileName, name)
		}
	}
	if !slices.Contains(dataDirs(), dir) {
		return "", fmt.Errorf("%w: directory %q", nodes.ErrFileName, dir)
	}
	flowID := "inline"
	if fl := rt.f.fs.flow; fl != nil {
		flowID = string(fl.ID)
	}
	parts := append([]string{rt.r.deps.DataDir(rt.f.fs.collection.Manifest.ID), dir, flowID}, segments...)
	file := filepath.Join(parts...)
	err := writeData(file, data, appendTo)
	if err != nil {
		return "", err
	}
	rt.r.addSaved([]string{file}, nil)
	return file, nil
}

func writeData(file string, data []byte, appendTo bool) error {
	if !appendTo {
		return writeFileAtomic(file, data)
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(file), err)
	}
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // path built from validated segments under the data dir
	if err != nil {
		return fmt.Errorf("opening %s: %w", file, err)
	}
	_, err = f.Write(data)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("appending to %s: %w", file, err)
	}
	return nil
}
