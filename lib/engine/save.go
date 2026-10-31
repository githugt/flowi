// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/githugt/flowi/lib/internal/names"
	"github.com/githugt/flowi/lib/vault"
)

// saveDir is where this request version's responses are kept.
func (x *execution) saveDir() string {
	key := ResponseKey(x.version.Label, x.c.overlayName())
	return filepath.Join(x.r.deps.DataDir(x.c.collection.Manifest.ID), "responses", string(x.c.request.ID), key)
}

// ResponseKey names the folder that keeps a version's saved responses. A send
// with a flowi overlay keeps its own, so a VIP response never overwrites the
// base one: "1" for the plain version, "1--vip" with the overlay vip. Version
// labels are numbers, so the double hyphen cannot be part of one.
func ResponseKey(version, overlay string) string {
	if overlay == "" {
		return version
	}
	return version + "--" + overlay
}

// saveRecord writes the masked record per the saving policy: none, last,
// or per-status (which also keeps last.json, so "last response" views
// always have something).
func saveRecord(rec *Record, dir, policy string, m vault.Masker) ([]string, error) {
	if rec == nil || policy == "none" || policy == "" {
		return nil, nil
	}
	data, err := json.MarshalIndent(rec.Masked(m), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encoding response record: %w", err)
	}
	names := []string{"last.json"}
	if policy == "per-status" {
		names = []string{statusFileName(rec) + ".json", "last.json"}
	}
	var written []string
	for _, name := range names {
		file := filepath.Join(dir, name)
		if err := writeFileAtomic(file, append(data, '\n')); err != nil {
			return written, err
		}
		written = append(written, file)
	}
	return written, nil
}

// statusFileName names the per-status file of a record: the status code,
// or the outcome for an async record (which has no status).
func statusFileName(rec *Record) string {
	if rec.HasMessages() {
		return rec.Outcome
	}
	return strconv.Itoa(rec.Status)
}

// example is a pinned example response: what impact analysis compares
// consumers against.
type example struct {
	Version string   `json:"version,omitempty"` // the request version it came from; unversioned examples apply to all
	Status  int      `json:"status"`
	Headers []Header `json:"headers,omitempty"`
	Body    any      `json:"body"`
}

// ErrExampleName is returned for a pin name that is not a plain file name.
var ErrExampleName = errors.New("example names use letters, digits, '.', '_' and '-', and do not start with '.'")

// ValidExampleName reports whether name can be a pinned example's file
// name: it must not reach outside the examples directory.
func ValidExampleName(name string) bool { return names.Valid(name) }

// PinExample writes rec as <request>.examples/<name>.json next to the
// request file (name defaults to the status code). rec must already be
// masked, as every record from a SendResult is.
func PinExample(rec Record, requestFile, name string) (string, error) {
	if name == "" {
		name = strconv.Itoa(rec.Status)
	}
	if !ValidExampleName(name) {
		return "", fmt.Errorf("pinning %q: %w", name, ErrExampleName)
	}
	var body any = rec.Body.Text
	if rec.Body.JSON != nil {
		body = rec.Body.JSON
	}
	data, err := json.MarshalIndent(example{Version: rec.Version, Status: rec.Status, Headers: rec.Header, Body: body}, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encoding example: %w", err)
	}
	file := ExampleFile(requestFile, name)
	if err := writeFileAtomic(file, append(data, '\n')); err != nil {
		return "", err
	}
	return file, nil
}

// autoPin pins a finished send as an example, as the responses.pin setting
// says (see PinExample): "first" only when the status has no example yet,
// "last" always. It returns the file written, "" when nothing was pinned.
// A record without a status code (a cancelled or failed connection) is not an
// example of anything.
func autoPin(rec *Record, requestFile, policy string, m vault.Masker) (string, error) {
	if rec == nil || requestFile == "" || (policy != "first" && policy != "last") || rec.Status == 0 {
		return "", nil
	}
	name := strconv.Itoa(rec.Status)
	if policy == "first" {
		if _, err := os.Stat(ExampleFile(requestFile, name)); err == nil {
			return "", nil
		}
	}
	return PinExample(rec.Masked(m), requestFile, name)
}

// ExampleFile is where the pinned example called name of a request file lives.
func ExampleFile(requestFile, name string) string {
	return filepath.Join(strings.TrimSuffix(requestFile, ".req.yaml")+".examples", name+".json")
}

// writeFileAtomic writes through a temp file and a rename, so a reader
// never sees half a file.
func writeFileAtomic(file string, data []byte) error {
	dir := filepath.Dir(file)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("writing %s: %w", file, err)
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck // removes the temp file only if the rename did not happen
	_, err = tmp.Write(data)
	if err == nil {
		err = tmp.Chmod(0o644)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), file)
	}
	if err != nil {
		return fmt.Errorf("writing %s: %w", file, err)
	}
	return nil
}
