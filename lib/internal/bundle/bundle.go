// SPDX-License-Identifier: Apache-2.0

// Package bundle is the file that holds one flow run so another person can open it: the flow and the requests
// it used, the seed and inputs, the report with the outcome of each step, and the masked record of every
// call. It is a zip, so any tool can look inside.
package bundle

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/report"
)

// Format is the version of the layout. A bundle with a higher one was made by a newer Flowi.
const Format = 1

// Extension is what a bundle file is named.
const Extension = ".flowi-run"

// Limits that keep a hostile file from filling the disk or memory: one entry, all entries, and their number.
const (
	maxEntry   = 64 << 20
	maxTotal   = 256 << 20
	maxEntries = 5000
)

// Manifest says what the run was.
type Manifest struct {
	Format     int               `json:"format"`
	Flowi      string            `json:"flowi,omitempty"` // the version that wrote the bundle
	Collection string            `json:"collection"`
	FlowID     model.ID          `json:"flow_id,omitempty"`
	FlowPath   string            `json:"flow_path"`
	FlowName   string            `json:"flow_name,omitempty"`
	Env        string            `json:"env,omitempty"`
	Seed       uint64            `json:"seed"`
	Inputs     map[string]string `json:"inputs,omitempty"`
	Status     string            `json:"status"`
	At         time.Time         `json:"at"`
	// Secrets are the names of the environment's secrets. Their values are never in a bundle; a replay
	// supplies the mask in their place so that every template still renders.
	Secrets []string `json:"secrets,omitempty"`
}

// Bundle is a run and what it needs to be shown again.
type Bundle struct {
	Manifest Manifest
	Report   report.Run
	Records  []engine.Record
	// Files are the collection's files by slash-separated path relative to its root: the manifest, flows and
	// requests, and nothing a run produced (examples, schemas, responses).
	Files map[string][]byte
	// Environment is the selected environment's file, with its secrets blank.
	Environment []byte
	// Sequence is the run drawn as a Mermaid sequence diagram, for anyone who opens the zip and wants the
	// picture. It is derived from the report and never read back.
	Sequence []byte
}

const (
	manifestName    = "bundle.json"
	reportName      = "report.json"
	recordsName     = "records.json"
	environmentName = "environment.yaml"
	sequenceName    = "sequence.mmd"
	filesPrefix     = "collection/"
)

// Write writes the bundle as a zip.
func Write(w io.Writer, b Bundle) error {
	b.Manifest.Format = Format
	zw := zip.NewWriter(w)
	put := func(name string, data []byte) error {
		f, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = f.Write(data)
		return err
	}
	parts := []struct {
		name  string
		value any
	}{{manifestName, b.Manifest}, {reportName, b.Report}, {recordsName, b.Records}}
	for _, p := range parts {
		data, err := json.MarshalIndent(p.value, "", "  ")
		if err != nil {
			return fmt.Errorf("encoding %s: %w", p.name, err)
		}
		if err := put(p.name, data); err != nil {
			return fmt.Errorf("writing %s: %w", p.name, err)
		}
	}
	if b.Environment != nil {
		if err := put(environmentName, b.Environment); err != nil {
			return fmt.Errorf("writing %s: %w", environmentName, err)
		}
	}
	if b.Sequence != nil {
		if err := put(sequenceName, b.Sequence); err != nil {
			return fmt.Errorf("writing %s: %w", sequenceName, err)
		}
	}
	for _, name := range sortedKeys(b.Files) {
		if err := put(filesPrefix+name, b.Files[name]); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
	}
	return zw.Close()
}

func sortedKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// ErrNotBundle is returned for a file that is not a bundle.
var ErrNotBundle = errors.New("this is not a Flowi run bundle")

// Read reads a bundle. Every entry name is checked and sizes are capped, because the file may come from anyone.
func Read(data []byte) (Bundle, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Bundle{}, fmt.Errorf("%w: %w", ErrNotBundle, err)
	}
	if len(zr.File) > maxEntries {
		return Bundle{}, fmt.Errorf("%w: it has %d entries", ErrNotBundle, len(zr.File))
	}
	entries, err := readEntries(zr)
	if err != nil {
		return Bundle{}, err
	}
	return assemble(entries)
}

// readEntries reads every entry whole, refusing unsafe names and oversized contents.
func readEntries(zr *zip.Reader) (map[string][]byte, error) {
	out := map[string][]byte{}
	total := 0
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if !safeName(f.Name) {
			return nil, fmt.Errorf("%w: unsafe entry name %q", ErrNotBundle, f.Name)
		}
		data, err := readEntry(f)
		if err != nil {
			return nil, err
		}
		if total += len(data); total > maxTotal {
			return nil, fmt.Errorf("%w: the contents are too large", ErrNotBundle)
		}
		out[f.Name] = data
	}
	return out, nil
}

func readEntry(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("%w: reading %s: %w", ErrNotBundle, f.Name, err)
	}
	defer rc.Close() //nolint:errcheck // a read-only entry
	data, err := io.ReadAll(io.LimitReader(rc, maxEntry+1))
	if err != nil {
		return nil, fmt.Errorf("%w: reading %s: %w", ErrNotBundle, f.Name, err)
	}
	if len(data) > maxEntry {
		return nil, fmt.Errorf("%w: %s is too large", ErrNotBundle, f.Name)
	}
	return data, nil
}

// safeName accepts only clean relative paths that stay inside the folder they are extracted to.
func safeName(name string) bool {
	if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, `\`) || strings.Contains(name, "//") {
		return false
	}
	return path.Clean(name) == name && !strings.HasPrefix(name, "../") && name != ".."
}

func assemble(entries map[string][]byte) (Bundle, error) {
	var b Bundle
	raw, ok := entries[manifestName]
	if !ok {
		return b, fmt.Errorf("%w: it has no %s", ErrNotBundle, manifestName)
	}
	if err := json.Unmarshal(raw, &b.Manifest); err != nil {
		return b, fmt.Errorf("%w: %s: %w", ErrNotBundle, manifestName, err)
	}
	if b.Manifest.Format > Format {
		return b, fmt.Errorf("this bundle is format %d, made by a newer Flowi (this one reads up to %d): update Flowi to open it", b.Manifest.Format, Format)
	}
	for name, into := range map[string]any{reportName: &b.Report, recordsName: &b.Records} {
		if data, ok := entries[name]; ok {
			if err := json.Unmarshal(data, into); err != nil {
				return b, fmt.Errorf("%w: %s: %w", ErrNotBundle, name, err)
			}
		}
	}
	b.Environment = entries[environmentName]
	b.Files = map[string][]byte{}
	for name, data := range entries {
		if rel, ok := strings.CutPrefix(name, filesPrefix); ok && rel != "" {
			b.Files[rel] = data
		}
	}
	return b, nil
}
