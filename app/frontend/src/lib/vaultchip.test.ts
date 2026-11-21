import { describe, expect, it } from 'vitest'
import { untilLock, vaultChip, type VaultState } from './vaultchip'

const open: VaultState = { exists: true, unlocked: true, prompts: true, autoLockSeconds: 900, locksInSeconds: 750 }

describe('vaultChip', () => {
  it('Given no vault file Then nothing is shown', () => {
    // Before: nothing in the window said whether the vault was open.
    expect(vaultChip({ ...open, exists: false })).toBeNull()
  })
  it('Given an open vault with auto-lock Then it says when it locks and can be locked', () => {
    const chip = vaultChip(open)

    expect(chip).toMatchObject({ text: 'Vault open · locks in 13 min', canLock: true })
  })
  it('Given an open vault with auto-lock off Then no countdown is shown', () => {
    expect(vaultChip({ ...open, autoLockSeconds: 0, locksInSeconds: 0 })?.text).toBe('Vault open')
  })
  it('Given a locked passphrase vault Then it says the passphrase is asked and Lock now is off', () => {
    const chip = vaultChip({ ...open, unlocked: false, locksInSeconds: 0 })

    expect(chip).toMatchObject({ text: 'Vault locked', canLock: false })
    expect(chip?.hint).toContain('asks for the passphrase')
  })
  it('Given a keychain vault Then locking is explained as dropping the key from memory', () => {
    expect(vaultChip({ ...open, prompts: false })?.hint).toContain('opens again by itself')
  })
})

describe('untilLock', () => {
  it('rounds minutes up and names the last minute', () => {
    expect(untilLock(61)).toBe('2 min')
    expect(untilLock(59)).toBe('under a minute')
  })
})
