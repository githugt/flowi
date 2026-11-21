import { describe, expect, it } from 'vitest'
import { extractChip } from './extractchip'

const response = { extracted: [{ name: 'token', value: 'abc', found: true }, { name: 'id', value: '', found: false }, { name: 'blank', value: '', found: true }] }

describe('extractChip', () => {
  it('should show the value that was taken, "(empty)" for an empty one, and "no match" when nothing matched', () => {
    // Before: the Extract tab never said whether a path matched anything.
    const taken = extractChip('token', response)
    const empty = extractChip('blank', response)
    const none = extractChip('id', response)

    expect(taken).toEqual({ state: 'ok', text: 'abc' })
    expect(empty).toEqual({ state: 'ok', text: '(empty)' })
    expect(none).toEqual({ state: 'none', text: 'no match' })
  })

  it('should show nothing for an extraction the send did not try, a blank name or no response', () => {
    const later = extractChip('later', response)
    const blank = extractChip(' ', response)
    const noResponse = extractChip('token', undefined)

    expect(later).toBeNull()
    expect(blank).toBeNull()
    expect(noResponse).toBeNull()
  })
})
