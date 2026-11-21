// SPDX-License-Identifier: Apache-2.0

package flowi

import "time"

// VaultState is what the window shows about the secrets vault.
type VaultState struct {
	Exists   bool // a vault file exists
	Unlocked bool // its key is in memory: secrets can be read without a prompt
	// Prompts says that opening it asks for the passphrase; otherwise it opens again by itself after a lock.
	Prompts bool
	// AutoLock is the vault.auto_lock setting: how long unused before it locks, 0 for never.
	AutoLock time.Duration
	// LocksIn is how long until it locks by itself, 0 when it is locked or AutoLock is off.
	LocksIn time.Duration
}

// VaultState reports whether the vault is open and when it will lock. It opens nothing and asks for nothing.
func (w *Workspace) VaultState() VaultState {
	st := w.secrets().Status()
	auto := w.vaultAutoLock()
	out := VaultState{Exists: st.Exists, Unlocked: st.Unlocked, Prompts: st.Prompts, AutoLock: auto}
	if st.Unlocked && auto > 0 {
		out.LocksIn = max(auto-st.Idle, time.Second)
	}
	return out
}

// LockVault forgets the vault's key now. The next use opens it again, asking for the passphrase when it has one.
func (w *Workspace) LockVault() { w.secrets().Lock() }

// AutoLockVault locks the vault when vault.auto_lock says it has been idle long enough, and says whether it did.
// A caller with a clock (the window) calls it from time to time.
func (w *Workspace) AutoLockVault() bool { return w.secrets().LockIfIdle(w.vaultAutoLock()) }

func (w *Workspace) vaultAutoLock() time.Duration {
	v, _ := w.Settings(Scope{}).Get("vault.auto_lock")
	d, _ := v.Value.(time.Duration)
	return d
}
