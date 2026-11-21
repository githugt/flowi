import { describe, expect, it } from 'vitest'
import { placeGuards } from './guards'

const g = (glyph: string) => ({ glyph, tip: glyph })

describe('placeGuards', () => {
  // Before: a guarded step looked like a plain one; no card drew its condition, error handling, limit or concurrency.
  it('puts the last chip against the right edge and the others to its left', () => {
    const { chips } = placeGuards([g('if'), g('∥')], 300, 14)

    expect(chips).toHaveLength(2)
    const last = chips[1]
    expect(last.x + last.w).toBe(300 - 14)
    expect(chips[0].x + chips[0].w).toBeLessThan(last.x)
  })
  it('takes no room when a step has no guards', () => {
    expect(placeGuards([], 300, 14)).toEqual({ chips: [], used: 0 })
    expect(placeGuards(undefined, 300, 14).used).toBe(0)
  })
  it('drops chips that would leave the sub-line no room', () => {
    const { chips } = placeGuards([g('if'), g('continue'), g('5s'), g('∥')], 150, 14)

    expect(chips.length).toBeLessThan(4)
    expect(chips.length).toBeGreaterThan(0)
  })
})
