// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// reportsDir writes one report file per status, oldest first, named like run ids (they sort by time).
func reportsDir(t *testing.T, statuses ...string) string {
	t.Helper()
	dir := t.TempDir()
	for i, s := range statuses {
		body := fmt.Sprintf(`{"id":"r%02d","status":%q}`, i, s)
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("r%02d.json", i)), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func left(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestPruneRuns_Given_more_reports_than_keep_When_pruned_Then_the_oldest_go(t *testing.T) {
	// Before: nothing ever deleted a report, so a schedule filled the disk.
	dir := reportsDir(t, "passed", "passed", "passed", "passed", "passed")

	removed, err := pruneRuns(dir, 2, 0, reportStatus, os.Remove)

	if err != nil || removed != 3 {
		t.Fatalf("removed %d, err %v", removed, err)
	}
	if got := left(t, dir); !slices.Equal(got, []string{"r03.json", "r04.json"}) {
		t.Fatalf("left %v", got)
	}
}

func TestPruneRuns_Given_old_failures_When_pruned_Then_the_newest_failures_beyond_keep_stay(t *testing.T) {
	// A failure from long ago is the one worth looking at; it must outlive the passing runs around it.
	dir := reportsDir(t, "failed", "passed", "error", "passed", "failed", "passed", "passed")

	removed, err := pruneRuns(dir, 2, 2, reportStatus, os.Remove)

	if err != nil || removed != 3 {
		t.Fatalf("removed %d, err %v", removed, err)
	}
	// Newest two (r05, r06) stay; of the rest the two newest failures (r04, r02) stay; r00 failed but is a third failure.
	if got := left(t, dir); !slices.Equal(got, []string{"r02.json", "r04.json", "r05.json", "r06.json"}) {
		t.Fatalf("left %v", got)
	}
}

func TestPruneRuns_Given_keep_zero_When_pruned_Then_nothing_is_deleted(t *testing.T) {
	dir := reportsDir(t, "passed", "passed", "passed")

	removed, err := pruneRuns(dir, 0, 0, reportStatus, os.Remove)

	if err != nil || removed != 0 || len(left(t, dir)) != 3 {
		t.Fatalf("removed %d, err %v, left %v", removed, err, left(t, dir))
	}
}

func TestPruneRuns_Given_other_files_and_a_missing_folder_When_pruned_Then_only_reports_are_touched(t *testing.T) {
	dir := reportsDir(t, "passed", "passed", "passed")
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := pruneRuns(dir, 1, 0, reportStatus, os.Remove)
	_, errMissing := pruneRuns(filepath.Join(dir, "nope"), 1, 0, reportStatus, os.Remove)

	if err != nil || errMissing != nil {
		t.Fatalf("errors %v %v", err, errMissing)
	}
	if got := left(t, dir); !slices.Equal(got, []string{"notes.txt", "r02.json"}) {
		t.Fatalf("left %v", got)
	}
}

func TestPruneRuns_Given_a_file_that_cannot_be_removed_When_pruned_Then_the_rest_still_go_and_the_error_is_returned(t *testing.T) {
	dir := reportsDir(t, "passed", "passed", "passed", "passed")
	stuck := filepath.Join(dir, "r00.json")
	remove := func(f string) error {
		if f == stuck {
			return fmt.Errorf("busy")
		}
		return os.Remove(f)
	}

	removed, err := pruneRuns(dir, 1, 0, reportStatus, remove)

	if err == nil || removed != 2 {
		t.Fatalf("removed %d, err %v", removed, err)
	}
}

func TestReportStatus_Given_an_unreadable_report_When_read_Then_it_does_not_count_as_passed(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "x.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	if reportStatus(bad) == "passed" || reportStatus(filepath.Join(dir, "gone.json")) == "passed" {
		t.Fatal("an unreadable report counted as passed")
	}
}
