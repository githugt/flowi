// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/internal/names"
)

// LastResponse is the response the last send of version of request t saved
// (responses.save "last" or "per-status"), so a restarted app can show it.
// ok is false when nothing was saved or the file cannot be read: saved
// responses are a cache nobody edits, so a bad one is not worth an error.
func (w *Workspace) LastResponse(t RequestTarget, version string) (rec engine.Record, ok bool) {
	c, r, err := w.requestFor(t)
	if err != nil || !names.Valid(version) {
		return engine.Record{}, false
	}
	file := filepath.Join(w.DataDir(c.Manifest.ID), "responses", string(r.ID), version, "last.json")
	data, err := os.ReadFile(file)
	if err != nil {
		return engine.Record{}, false
	}
	if json.Unmarshal(data, &rec) != nil {
		return engine.Record{}, false
	}
	return rec, true
}
