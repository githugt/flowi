import { describe, expect, it } from 'vitest'
import { alreadyShown } from './codeecho'

describe('alreadyShown', () => {
  it('answers from the last reported text without reading the document', () => {
    let reads = 0

    const same = alreadyShown('abc', 'abc', () => (reads++, 'abc'))

    // Before: the document was always turned into a string to compare.
    expect(same).toBe(true)
    expect(reads).toBe(0)
  })
  it('reads the document when the text differs from the last reported one', () => {
    expect(alreadyShown('abc', 'abd', () => 'abd')).toBe(true)
    expect(alreadyShown('abc', 'abd', () => 'abc')).toBe(false)
  })
  it('reads the document when nothing was reported yet', () => {
    expect(alreadyShown(null, 'x', () => 'x')).toBe(true)
    expect(alreadyShown(null, 'x', () => 'y')).toBe(false)
  })
})
