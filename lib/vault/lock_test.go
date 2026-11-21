// SPDX-License-Identifier: Apache-2.0

package vault

import (
	"errors"
	"testing"
	"time"
)

// phraseVault is a vault with a passphrase, a fake clock and a count of the prompts.
func phraseVault(t *testing.T) (*Vault, *time.Time, *int) {
	t.Helper()
	now := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	prompts := 0
	o := options(t, nil)
	o.Now = func() time.Time { return now }
	o.Passphrase = func() (string, error) { prompts++; return "correct horse", nil }
	o.NewPassphrase = o.Passphrase
	return New(o), &now, &prompts
}

func TestStatus_Given_no_vault_Then_nothing_exists_and_nothing_is_read(t *testing.T) {
	v, _, prompts := phraseVault(t)

	got := v.Status()

	if got.Exists || got.Unlocked || *prompts != 0 {
		t.Fatalf("status %+v, prompts %d", got, *prompts)
	}
}

func TestLock_Given_a_passphrase_vault_Then_the_next_use_asks_for_the_passphrase_again(t *testing.T) {
	// Before: once opened, the key stayed in memory for as long as the app ran.
	v, _, prompts := phraseVault(t)
	if err := v.Set(testEntry(), "hunter2"); err != nil {
		t.Fatal(err)
	}
	open := v.Status()

	v.Lock()
	locked := v.Status()
	value, ok, err := v.Get(testEntry(), "local")

	if !open.Exists || !open.Unlocked || !open.Prompts || locked.Unlocked || !locked.Exists {
		t.Fatalf("open %+v, locked %+v", open, locked)
	}
	if err != nil || !ok || value != "hunter2" || *prompts != 2 {
		t.Fatalf("value %q ok %v err %v prompts %d (one to create, one to reopen)", value, ok, err, *prompts)
	}
}

func TestLock_Given_a_wrong_passphrase_after_a_lock_Then_it_stays_locked(t *testing.T) {
	v, _, _ := phraseVault(t)
	if err := v.Set(testEntry(), "hunter2"); err != nil {
		t.Fatal(err)
	}
	v.Lock()
	v.opts.Passphrase = func() (string, error) { return "wrong", nil }

	_, _, err := v.Get(testEntry(), "local")

	var locked *LockedError
	if !errors.As(err, &locked) || v.Status().Unlocked {
		t.Fatalf("err %v, status %+v", err, v.Status())
	}
}

func TestLockIfIdle_Given_use_Then_it_locks_only_after_the_idle_time_with_no_use(t *testing.T) {
	v, now, _ := phraseVault(t)
	if err := v.Set(testEntry(), "hunter2"); err != nil {
		t.Fatal(err)
	}

	*now = now.Add(10 * time.Minute)
	early := v.LockIfIdle(15 * time.Minute)
	idle := v.Status().Idle
	*now = now.Add(6 * time.Minute) // 16 minutes since the Set: nothing used it in between
	late := v.LockIfIdle(15 * time.Minute)

	if early || idle != 10*time.Minute || !late || v.Status().Unlocked {
		t.Fatalf("early %v idle %v late %v status %+v", early, idle, late, v.Status())
	}
}

func TestLockIfIdle_Given_a_use_in_between_Then_the_idle_time_starts_again(t *testing.T) {
	v, now, _ := phraseVault(t)
	if err := v.Set(testEntry(), "hunter2"); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(14 * time.Minute)
	if _, _, err := v.Get(testEntry(), "local"); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(14 * time.Minute)

	locked := v.LockIfIdle(15 * time.Minute)

	if locked || !v.Status().Unlocked {
		t.Fatalf("locked %v, status %+v", locked, v.Status())
	}
}

func TestLockIfIdle_Given_zero_or_a_locked_vault_Then_it_does_nothing(t *testing.T) {
	v, now, _ := phraseVault(t)
	if err := v.Set(testEntry(), "hunter2"); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(time.Hour)

	off := v.LockIfIdle(0)
	v.Lock()
	again := v.LockIfIdle(time.Minute)

	if off || again {
		t.Fatalf("off %v, again %v", off, again)
	}
}

func TestStatus_Given_a_key_in_the_environment_Then_opening_does_not_prompt(t *testing.T) {
	o := options(t, nil)
	o.Environ = []string{"FLOWI_VAULT_KEY=test-only"}
	v := New(o)
	if err := v.Set(testEntry(), "x"); err != nil {
		t.Fatal(err)
	}

	got := v.Status()

	if got.Prompts {
		t.Fatalf("status %+v", got)
	}
}
