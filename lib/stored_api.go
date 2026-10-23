// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/internal/names"
)

// What a stored response is.
const (
	StoredSaved   = "saved"   // a response a send or the recorder kept, in the workspace data folder
	StoredExample = "example" // a pinned example, next to the request file and committed with the collection
)

// StoredResponse is a response kept for a request.
type StoredResponse struct {
	Kind    string // StoredSaved or StoredExample
	Version string // the request version it is for; "" for an example that fits every version
	Name    string // the file name without .json: a status ("201"), an outcome, "last", or the name of an example
	Status  int    // 0 when the file does not say
	At      time.Time
	Size    int64
}

// StoredResponses lists what is kept for request t: pinned examples first,
// then saved responses, each by version and name. A file that cannot be read
// is listed without a status: it can still be deleted.
func (w *Workspace) StoredResponses(t RequestTarget) ([]StoredResponse, error) {
	c, r, err := w.requestFor(t)
	if err != nil {
		return nil, err
	}
	out := listExamples(r.File)
	versions, _ := os.ReadDir(filepath.Join(w.DataDir(c.Manifest.ID), "responses", string(r.ID)))
	for _, v := range versions {
		if v.IsDir() && names.Valid(v.Name()) {
			out = append(out, listDir(filepath.Join(w.DataDir(c.Manifest.ID), "responses", string(r.ID), v.Name()), StoredSaved, v.Name())...)
		}
	}
	return out, nil
}

// listExamples lists the pinned examples of a request file.
func listExamples(requestFile string) []StoredResponse {
	if requestFile == "" {
		return nil
	}
	return listDir(strings.TrimSuffix(requestFile, ".req.yaml")+".examples", StoredExample, "")
}

func listDir(dir, kind, version string) []StoredResponse {
	entries, _ := os.ReadDir(dir) // no directory is the normal case
	var out []StoredResponse
	for _, e := range entries {
		name, isJSON := strings.CutSuffix(e.Name(), ".json")
		if e.IsDir() || !isJSON || !names.Valid(name) {
			continue
		}
		s := StoredResponse{Kind: kind, Version: version, Name: name}
		if info, err := e.Info(); err == nil {
			s.Size, s.At = info.Size(), info.ModTime()
		}
		s.Status, s.Version, s.At = describeStored(filepath.Join(dir, e.Name()), version, s.At)
		out = append(out, s)
	}
	return out
}

// describeStored reads the status, the version (an example names its own) and
// the time (a record has its own) of a stored file; what it cannot read stays
// as it was.
func describeStored(file, version string, at time.Time) (status int, ver string, when time.Time) {
	data, err := os.ReadFile(file)
	if err != nil {
		return 0, version, at
	}
	var f struct {
		Status  int       `json:"status"`
		Version string    `json:"version"`
		At      time.Time `json:"at"`
	}
	if json.Unmarshal(data, &f) != nil {
		return 0, version, at
	}
	if version == "" {
		version = f.Version
	}
	if !f.At.IsZero() {
		at = f.At
	}
	return f.Status, version, at
}

// ErrStoredKind is returned for a kind that is not StoredSaved or StoredExample.
var ErrStoredKind = errors.New("a stored response is saved or an example")

// storedFile is where a stored response lives, after checking every part of
// the name, so a caller cannot reach outside the folders of the request.
func (w *Workspace) storedFile(t RequestTarget, kind, version, name string) (string, error) {
	c, r, err := w.requestFor(t)
	if err != nil {
		return "", err
	}
	if !names.Valid(name) {
		return "", fmt.Errorf("%q is not a plain file name", name)
	}
	switch kind {
	case StoredExample:
		return engine.ExampleFile(r.File, name), nil
	case StoredSaved:
		if !names.Valid(version) {
			return "", fmt.Errorf("%q is not a plain version label", version)
		}
		return filepath.Join(w.DataDir(c.Manifest.ID), "responses", string(r.ID), version, name+".json"), nil
	}
	return "", ErrStoredKind
}

// DeleteStoredResponse removes one stored response. A response that is
// already gone is not an error.
func (w *Workspace) DeleteStoredResponse(t RequestTarget, kind, version, name string) error {
	file, err := w.storedFile(t, kind, version, name)
	if err != nil {
		return err
	}
	if err := os.Remove(file); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("removing %s: %w", filepath.Base(file), err)
	}
	return nil
}

// StoredRecord reads a saved response as the record it was, to show it again.
// Pinned examples are not records and are not read here.
func (w *Workspace) StoredRecord(t RequestTarget, version, name string) (engine.Record, error) {
	file, err := w.storedFile(t, StoredSaved, version, name)
	if err != nil {
		return engine.Record{}, err
	}
	data, err := os.ReadFile(file) //nolint:gosec // storedFile checked every part of the name
	if err != nil {
		return engine.Record{}, fmt.Errorf("reading the saved response: %w", err)
	}
	var rec engine.Record
	if err := json.Unmarshal(data, &rec); err != nil {
		return engine.Record{}, fmt.Errorf("the saved response %s is damaged: %w", name, err)
	}
	return rec, nil
}
