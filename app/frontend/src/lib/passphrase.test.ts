import { describe, expect, it } from 'vitest'
import { checkNewPassphrase, newPassphraseProblem } from './passphrase'

describe('checkNewPassphrase', () => {
  it('accepts two equal entries', () => {
    expect(checkNewPassphrase('correct horse', 'correct horse')).toBe('ok')
  })
  it('refuses entries that differ, so a typo cannot lock the vault', () => {
    expect(checkNewPassphrase('correct horse', 'correct hrose')).toBe('mismatch')
  })
  it('refuses an empty passphrase', () => {
    expect(checkNewPassphrase('', '')).toBe('empty')
    expect(checkNewPassphrase('   ', '   ')).toBe('empty')
  })
})

describe('newPassphraseProblem', () => {
  it('says what is wrong, and nothing when it is fine', () => {
    expect(newPassphraseProblem('mismatch')).toContain('not the same')
    expect(newPassphraseProblem('empty')).toContain('empty')
    expect(newPassphraseProblem('ok')).toBe('')
  })
})
