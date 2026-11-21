// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"slices"
	"testing"
	"time"

	flowi "github.com/githugt/flowi/lib"
)

func TestSecrets_Load_Given_an_open_vault_Then_its_state_and_countdown_are_shown(t *testing.T) {
	// Before: nothing in the window said whether the vault was open.
	b := secretsBackend()
	b.Vault = flowi.VaultState{Exists: true, Unlocked: true, AutoLock: 15 * time.Minute, LocksIn: 12 * time.Minute}

	s, _ := newSecrets(t, b)

	if got := s.Vault(); !got.Unlocked || got.LocksIn != 12*time.Minute {
		t.Fatalf("vault %+v", got)
	}
}

func TestSecrets_Lock_Then_the_vault_is_locked_and_the_secrets_are_not_read_again(t *testing.T) {
	// Before: reading the list after a lock would have opened the vault again, asking for the passphrase at once.
	b := secretsBackend()
	b.Vault = flowi.VaultState{Exists: true, Unlocked: true, Prompts: true}
	s, w := newSecrets(t, b)
	reads := b.SecretsReads

	s.Lock()
	w.Wait()

	if s.Vault().Unlocked || !slices.Contains(b.Calls, "LockVault") || b.SecretsReads != reads {
		t.Fatalf("vault %+v, calls %v", s.Vault(), b.Calls)
	}
}

func TestSecrets_Tick_Given_the_vault_idled_out_Then_it_locks_and_the_change_is_announced_once(t *testing.T) {
	b := secretsBackend()
	b.Vault = flowi.VaultState{Exists: true, Unlocked: true, AutoLock: time.Minute, LocksIn: time.Second}
	s, w := newSecrets(t, b)
	fired := 0
	s.OnChange(func() { fired++ })

	s.Tick() // not idle yet: the countdown is the same
	w.Wait()
	quiet := fired
	b.VaultIdledOut = true
	s.Tick()
	w.Wait()
	s.Tick() // already locked: nothing changes
	w.Wait()

	if quiet != 0 || fired != 1 || s.Vault().Unlocked {
		t.Fatalf("quiet %d fired %d vault %+v", quiet, fired, s.Vault())
	}
}
