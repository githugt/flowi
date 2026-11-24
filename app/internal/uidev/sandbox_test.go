// SPDX-License-Identifier: AGPL-3.0-or-later

package uidev

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Before: each run that imported anything left its sandbox folder in the temp directory for good.
func TestSweepSandboxes_Given_old_and_young_folders_Then_only_the_old_ones_with_the_prefix_go(t *testing.T) {
	dir := t.TempDir()
	old, young, other := filepath.Join(dir, sandboxPrefix+"old"), filepath.Join(dir, sandboxPrefix+"young"), filepath.Join(dir, "something-else")
	for _, d := range []string{old, young, other} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	past := time.Now().Add(-2 * staleSandboxAge)
	for _, d := range []string{old, other} {
		if err := os.Chtimes(d, past, past); err != nil {
			t.Fatal(err)
		}
	}

	removed := sweepSandboxes(dir, staleSandboxAge, time.Now())

	_, errOld := os.Stat(old)
	_, errYoung := os.Stat(young)
	_, errOther := os.Stat(other)
	if removed != 1 || errOld == nil || errYoung != nil || errOther != nil {
		t.Fatalf("removed %d, old %v, young %v, other %v", removed, errOld, errYoung, errOther)
	}
}

func TestBackend_Close_removes_the_sandbox_folder_once_one_was_made(t *testing.T) {
	b := New()
	if err := b.Close(); err != nil {
		t.Fatal(err) // nothing was made: nothing to remove
	}
	if _, _, err := b.sandbox.open(); err != nil {
		t.Fatal(err)
	}
	dir := b.sandbox.dir

	if err := b.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(dir); err == nil {
		t.Fatalf("%s is still there", dir)
	}
}
