// The line about the secrets vault: open or locked, when it locks by itself, and whether it can be locked now.
// Before: nothing in the window said whether the vault was open, and nothing could lock it.

export interface VaultState {
  exists: boolean
  unlocked: boolean
  /** Opening it asks for the passphrase; otherwise it opens again by itself after a lock. */
  prompts: boolean
  autoLockSeconds: number
  locksInSeconds: number
}

export interface VaultChip {
  text: string
  /** Say more, as a tooltip or a second line. */
  hint: string
  canLock: boolean
}

/** "12 min", or "under a minute" for what is left of the last one. */
export function untilLock(seconds: number): string {
  if (seconds < 60) return 'under a minute'
  return `${Math.ceil(seconds / 60)} min`
}

/** What to show, or null while no vault exists yet (nothing is stored, so there is nothing to lock). */
export function vaultChip(v: VaultState): VaultChip | null {
  if (!v.exists) return null
  if (!v.unlocked) {
    return {
      text: 'Vault locked',
      hint: v.prompts ? 'The next time a secret is needed, Flowi asks for the passphrase.' : 'It opens again by itself when a secret is needed.',
      canLock: false,
    }
  }
  const left = v.autoLockSeconds > 0 && v.locksInSeconds > 0 ? ` · locks in ${untilLock(v.locksInSeconds)}` : ''
  return {
    text: `Vault open${left}`,
    hint: v.prompts
      ? 'Locking forgets the key; the next use asks for the passphrase. Set vault.auto_lock to lock after a time without use.'
      : 'Locking drops the key from memory; it opens again by itself (the key is in the system keychain or FLOWI_VAULT_KEY). Set vault.auto_lock to lock after a time without use.',
    canLock: true,
  }
}
