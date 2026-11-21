// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"bytes"
	"errors"
	flowi "github.com/githugt/flowi/lib"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func TestSecretsService_Reveal_should_return_one_value_and_log_that_it_happened_without_the_value(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	b := &vmtest.Backend{SecretValues: map[string]string{"local/password": "s3cr3t-pw"}}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit, bridge.WithLogger(slog.New(slog.NewTextHandler(&logs, nil))))
	app.Start(t.Context())

	got, err := app.Secrets().Reveal("local", "password")

	if err != nil || got != "s3cr3t-pw" {
		t.Fatalf("got %q %v", got, err)
	}
	if !strings.Contains(logs.String(), "SecretRevealed") || strings.Contains(logs.String(), "s3cr3t-pw") {
		t.Fatalf("log %q", logs.String())
	}
}

func TestSecretsService_Reveal_Given_no_value_Then_an_error_and_no_log_of_a_reveal(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	app := bridge.NewApp(&vmtest.Backend{}, bridge.NewInlineActor(), (&published{}).emit, bridge.WithLogger(slog.New(slog.NewTextHandler(&logs, nil))))
	app.Start(t.Context())

	got, err := app.Secrets().Reveal("local", "password")

	if err == nil || got != "" || strings.Contains(logs.String(), "SecretRevealed") {
		t.Fatalf("got %q %v log %q", got, err, logs.String())
	}
}

func TestSecretsService_State_Given_values_were_revealed_Then_the_view_counts_only_the_ones_that_were_shown(t *testing.T) {
	// Before: nothing counted how often a secret was shown in a session.
	t.Parallel()
	b := &vmtest.Backend{SecretValues: map[string]string{"local/password": "s3cr3t-pw"}}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())
	secrets := app.Secrets()

	_, _ = secrets.Reveal("local", "password")
	_, _ = secrets.Reveal("local", "password")
	_, _ = secrets.Reveal("local", "missing") // a failed reveal shows nothing

	if got := secrets.State().Reveals; got != 2 {
		t.Fatalf("Reveals = %d, want 2", got)
	}
}

// Before: Set and Unset dropped the vault's answer, so a refusal was silent and the screen said "Stored" anyway.
func TestSecretsService_Set_and_Unset_Given_the_vault_refuses_Then_the_error_comes_back(t *testing.T) {
	t.Parallel()
	b := &vmtest.Backend{SecretErr: errors.New("the vault is locked")}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())

	set := app.Secrets().Set("staging", "password", "x")
	unset := app.Secrets().Unset("staging", "password")

	if set == nil || unset == nil || !strings.Contains(set.Error(), "locked") || !strings.Contains(unset.Error(), "locked") {
		t.Fatalf("set %v unset %v", set, unset)
	}
}

func TestSecretsService_Set_Given_the_vault_accepts_Then_no_error(t *testing.T) {
	t.Parallel()
	app := bridge.NewApp(&vmtest.Backend{}, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())

	err := app.Secrets().Set("staging", "password", "x")

	if err != nil {
		t.Fatalf("err %v", err)
	}
}

// Before: the Secrets screen showed a name and a state, and nothing said what the secret was used by.
func TestSecretsService_State_should_list_the_files_that_use_each_secret_with_the_node_that_opens_them(t *testing.T) {
	t.Parallel()
	b := &vmtest.Backend{SecretList: []flowi.SecretInfo{{Environment: "staging", Name: "password", Set: true}}, Uses: map[string][]flowi.SecretUse{"password": {
		{Kind: flowi.UseAuth, Name: "partner-api"},
		{Kind: flowi.UseRequest, Name: "auth/login", Collection: "C1", ID: "R1"},
		{Kind: flowi.UseFlow, Name: "signup", Collection: "C1", ID: "F1"},
	}}}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())

	var pw bridge.SecretView
	for _, s := range app.Secrets().State().Secrets {
		if s.Name == "password" {
			pw = s
		}
	}

	if len(pw.UsedBy) != 3 || pw.UsedBy[0].Node != "" || pw.UsedBy[1].Node != "r:C1:R1" || pw.UsedBy[2].Node != "f:C1:F1" {
		t.Fatalf("used by %+v", pw.UsedBy)
	}
}

func TestSecretsService_State_Given_nothing_uses_a_secret_Then_the_list_is_empty_not_null(t *testing.T) {
	t.Parallel()
	app := bridge.NewApp(&vmtest.Backend{SecretList: []flowi.SecretInfo{{Environment: "staging", Name: "password"}}}, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())

	rows := app.Secrets().State().Secrets
	if len(rows) != 1 {
		t.Fatalf("rows %+v", rows)
	}
	for _, s := range rows {
		if s.UsedBy == nil {
			t.Fatalf("%s has a nil UsedBy", s.Name)
		}
	}
}

func TestSecretsService_State_Given_an_open_vault_Then_the_view_says_so_and_LockVault_locks_it_without_reading_the_secrets(t *testing.T) {
	// Before: nothing in the window said whether the vault was open, and nothing could lock it.
	t.Parallel()
	b := &vmtest.Backend{Vault: flowi.VaultState{Exists: true, Unlocked: true, Prompts: true, AutoLock: 15 * time.Minute, LocksIn: 12 * time.Minute}}
	pub := &published{}
	app := bridge.NewApp(b, bridge.NewInlineActor(), pub.emit)
	app.Start(t.Context())
	secrets := app.Secrets()

	open := secrets.State().Vault
	reads := b.SecretsReads
	secrets.LockVault()
	locked := secrets.State().Vault

	if !open.Unlocked || !open.Prompts || open.AutoLockSeconds != 900 || open.LocksInSeconds != 720 {
		t.Fatalf("open %+v", open)
	}
	if locked.Unlocked || locked.LocksInSeconds != 0 || b.SecretsReads != reads+1 { // the State call after the lock reads once
		t.Fatalf("locked %+v reads %d (was %d)", locked, b.SecretsReads, reads)
	}
}
