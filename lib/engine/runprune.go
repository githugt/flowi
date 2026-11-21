// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// pruneRuns deletes the oldest report files in dir (one flow's runs folder) beyond keep. Of the ones beyond keep, the
// newest keepFailed whose run did not pass are kept too, so a failure from long ago is still there to look at. keep 0
// keeps everything. Run ids are time-ordered, so the file names sort oldest first. It returns how many it deleted, and
// the errors of the deletions that failed; a file that cannot be deleted does not stop the rest.
//
// Before: the run history was never deleted, so a schedule that ran every minute filled the disk.
func pruneRuns(dir string, keep, keepFailed int, statusOf func(file string) string, remove func(file string) error) (int, error) {
	if keep <= 0 {
		return 0, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, nil // no folder, no reports
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	if len(names) <= keep {
		return 0, nil
	}
	slices.Sort(names)
	slices.Reverse(names) // newest first
	removed, failedKept := 0, 0
	var errs []error
	for _, name := range names[keep:] {
		file := filepath.Join(dir, name)
		if failedKept < keepFailed && statusOf(file) != "passed" {
			failedKept++
			continue
		}
		if err := remove(file); err != nil {
			errs = append(errs, err)
			continue
		}
		removed++
	}
	return removed, errors.Join(errs...)
}

// reportStatus is the status a saved report file says the run ended with; "" for a file that cannot be read, which
// counts as not passed and so is kept among the failures rather than deleted unseen.
func reportStatus(file string) string {
	data, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	var r struct {
		Status string `json:"status"`
	}
	if json.Unmarshal(data, &r) != nil {
		return ""
	}
	return r.Status
}
