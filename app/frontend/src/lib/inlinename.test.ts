import { describe, expect, it } from 'vitest'
import { inlineKey, nameToKeep, shownName } from './inlinename'

describe('inlineKey', () => {
  it('Given Enter or Escape When pressed Then it commits or reverts', () => {
    expect(inlineKey('Enter')).toBe('commit')
    expect(inlineKey('Escape')).toBe('revert')
    expect(inlineKey('a')).toBe('')
  })
  it('Given a key that picks a character in an input method When pressed Then it is not a command', () => {
    expect(inlineKey('Enter', true)).toBe('')
  })
})

describe('nameToKeep', () => {
  it('Given a changed name When leaving the field Then it is kept, trimmed', () => {
    // Before: only Enter kept it, so clicking away lost it.
    expect(nameToKeep('  login2 ', 'login')).toBe('login2')
  })
  it('Given the same or an empty name When leaving the field Then there is nothing to keep', () => {
    expect(nameToKeep('login', 'login')).toBeNull()
    expect(nameToKeep(' login ', 'login')).toBeNull()
    expect(nameToKeep('  ', 'login')).toBeNull()
  })
})

describe('shownName', () => {
  it('Given editing over When shown Then it is the name itself', () => {
    expect(shownName('draft', 'login', true)).toBe('draft')
    expect(shownName('draft', 'login', false)).toBe('login')
  })
})
