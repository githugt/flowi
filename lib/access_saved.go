// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/githugt/flowi/lib/internal/names"
)

// SavedAccess is the last access check of a flow and when it ran.
type SavedAccess struct {
	At     time.Time
	Report AccessReport
}

// accessFile is where the flow's last access check is kept: the data folder, not the collection, because it is a
// result of running and not something to commit. "" when the flow's id is not a plain name.
func (w *Workspace) accessFile(t FlowTarget) string {
	if !names.Valid(string(t.Flow)) || !names.Valid(string(t.Collection)) {
		return ""
	}
	return filepath.Join(w.DataDir(t.Collection), "access", string(t.Flow)+".json")
}

// saveAccess keeps rep as the flow's last access check, replacing the one before. The file is a cache of a result,
// so a failure to write it does not fail the check that produced it; the check then just is not there next time.
//
// Before: the table of results was lost when the dialog closed, and nothing said when a check was last run.
func (w *Workspace) saveAccess(t FlowTarget, rep AccessReport, at time.Time) {
	file := w.accessFile(t)
	if file == "" {
		return
	}
	data, err := json.MarshalIndent(SavedAccess{At: at, Report: rep}, "", "  ")
	if err != nil || os.MkdirAll(filepath.Dir(file), 0o750) != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), ".access-*")
	if err != nil {
		return
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Rename(tmp.Name(), file) != nil {
		_ = os.Remove(tmp.Name())
	}
}

// LastAccess is the last access check run for the flow, if one was kept.
func (w *Workspace) LastAccess(t FlowTarget) (SavedAccess, bool) {
	file := w.accessFile(t)
	if file == "" {
		return SavedAccess{}, false
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return SavedAccess{}, false
	}
	var s SavedAccess
	if json.Unmarshal(data, &s) != nil || s.At.IsZero() {
		return SavedAccess{}, false
	}
	return s, true
}
