// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"strings"
	"testing"
	"time"
)

func TestStaleNotes_should_name_frozen_values_that_expired_and_skip_the_ones_the_fork_changes(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	frozen := map[string]any{
		"expires_at":    now.Add(-2 * time.Hour).Format(time.RFC3339),
		"other_expires": now.Add(-time.Hour).Format(time.RFC3339),
		"fresh_expires": now.Add(time.Hour).Format(time.RFC3339),
		"city":          "Paris",
	}

	got := staleNotes(frozen, map[string]any{"other_expires": "later"}, now)

	if len(got) != 1 || !strings.HasPrefix(got[0], "expires_at is stale") {
		t.Fatalf("notes %v: want only expires_at", got)
	}
}
