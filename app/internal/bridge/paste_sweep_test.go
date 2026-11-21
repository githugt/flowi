// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSweepPasted_Given_old_and_new_paste_files_Then_only_the_old_ones_go(t *testing.T) {
	// Before: a pasted collection stayed in the temp folder after a crash, for good.
	dir := t.TempDir()
	now := time.Now()
	old := filepath.Join(dir, "flowi-paste-old")
	fresh := filepath.Join(dir, "flowi-paste-new")
	other := filepath.Join(dir, "somebody-elses-old-file")
	for _, f := range []string{old, fresh, other} {
		if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{old, other} {
		if err := os.Chtimes(f, now.Add(-24*time.Hour), now.Add(-24*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}

	removed := sweepPasted(dir, 12*time.Hour, now)

	if removed != 1 {
		t.Fatalf("removed %d, want 1", removed)
	}
	for f, want := range map[string]bool{old: false, fresh: true, other: true} {
		if _, err := os.Stat(f); (err == nil) != want {
			t.Errorf("%s: exists %v, want %v", filepath.Base(f), err == nil, want)
		}
	}
}
