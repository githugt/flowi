import { describe, expect, it } from 'vitest'
import { isLetter } from './keymatch'

describe('isLetter', () => {
  it('matches the letter in either case', () => {
    expect(isLetter({ key: 's', code: 'KeyS' }, 's')).toBe(true)
    expect(isLetter({ key: 'S', code: 'KeyS' }, 's')).toBe(true)
  })
  it('matches by the key place when the layout has no Latin letters', () => {
    expect(isLetter({ key: 'ы', code: 'KeyS' }, 's')).toBe(true) // Russian
    expect(isLetter({ key: 'σ', code: 'KeyS' }, 's')).toBe(true) // Greek
  })
  it('follows the letter on a Latin layout that moved it', () => {
    expect(isLetter({ key: 'o', code: 'KeyS' }, 's')).toBe(false) // Dvorak: that key types o
    expect(isLetter({ key: 's', code: 'KeyO' }, 's')).toBe(true)
  })
  it('does not match another letter or an unrelated key', () => {
    expect(isLetter({ key: 'z', code: 'KeyZ' }, 's')).toBe(false)
    expect(isLetter({ key: 'Enter', code: 'Enter' }, 's')).toBe(false)
    expect(isLetter({ key: 'я', code: 'KeyZ' }, 's')).toBe(false)
  })
})
