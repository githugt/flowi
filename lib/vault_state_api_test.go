// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"testing"
	"time"

	"github.com/githugt/flowi/lib/settings"
)

func TestVaultState_Given_auto_lock_Then_it_counts_down_and_locks_after_the_idle_time_and_reopens_on_use(t *testing.T) {
	// Before: the vault had no visible state and the key stayed in memory for as long as the app ran.
	now := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	w := demoWorkspaceWith(t, func(o *WorkspaceOptions) {
		o.Now = func() time.Time { return now }
		o.Flags = settings.LayerValues{Layer: settings.LayerFlag, Source: "--vault-auto-lock", Values: map[string]any{"vault.auto_lock": 15 * time.Minute}}
	})
	open := w.VaultState() // the setup stored a secret, which opened it

	now = now.Add(10 * time.Minute)
	counting := w.VaultState()
	early := w.AutoLockVault()
	now = now.Add(6 * time.Minute)
	locked := w.AutoLockVault()
	after := w.VaultState()
	_, err := w.Secrets() // a use opens it again by itself: its key is in the keychain
	reopened := w.VaultState()

	if !open.Exists || !open.Unlocked || open.AutoLock != 15*time.Minute || open.Prompts {
		t.Fatalf("open %+v", open)
	}
	if counting.LocksIn != 5*time.Minute || early {
		t.Fatalf("counting %+v early %v", counting, early)
	}
	if !locked || after.Unlocked || after.LocksIn != 0 || !after.Exists {
		t.Fatalf("locked %v after %+v", locked, after)
	}
	if err != nil || !reopened.Unlocked {
		t.Fatalf("err %v reopened %+v", err, reopened)
	}
}

func TestLockVault_Given_an_open_vault_Then_it_is_locked_at_once_and_auto_lock_off_never_locks(t *testing.T) {
	w := demoWorkspace(t)

	off := w.AutoLockVault()
	w.LockVault()

	if off || w.VaultState().Unlocked || w.VaultState().AutoLock != 0 {
		t.Fatalf("off %v state %+v", off, w.VaultState())
	}
}
