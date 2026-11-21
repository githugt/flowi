// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"testing"

	flowi "github.com/githugt/flowi/lib"
)

func TestMissingSecrets(t *testing.T) {
	// Before: nothing told CI which secrets had no value until a request needed one.
	rows := []secretRow{
		{Environment: "staging", Name: "token", State: "set"},
		{Environment: "staging", Name: "password", State: "missing"},
		{Environment: "prod", Name: "token", State: "env override"},
		{Environment: "prod", Name: "key", State: "missing"},
	}

	got := missingSecrets(rows)

	if len(got) != 2 || got[0].Name != "password" || got[1].Name != "key" {
		t.Fatalf("got %+v, want the two missing rows in order", got)
	}
	if len(missingSecrets(rows[:1])) != 0 {
		t.Fatal("a set secret must not be reported")
	}
}

func TestOrphanRows(t *testing.T) {
	// Before: a secret whose environment or variable was gone stayed in the vault with no way to list or remove it.
	found := []flowi.OrphanSecret{{Environment: "local", Name: "password"}, {Environment: "01JZ", Name: "token", EnvironmentGone: true}}

	removed := orphanRows(found, false)
	listed := orphanRows(found, true)

	if removed[0].State != "removed" || removed[1].State != "removed (environment gone)" {
		t.Errorf("removed: %+v", removed)
	}
	if listed[0].State != "orphan" || listed[1].State != "orphan (environment gone)" {
		t.Errorf("listed: %+v", listed)
	}
	if len(orphanRows(nil, true)) != 0 {
		t.Error("nothing found must give no rows")
	}
}
