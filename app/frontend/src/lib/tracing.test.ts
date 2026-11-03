import { describe, expect, it } from 'vitest'
import { agoText } from './tracing'

const at = '2026-11-03T10:00:00Z'
const after = (s: number) => Date.parse(at) + s * 1000

describe('agoText', () => {
  it.each([
    [1, 'just now'],
    [30, '30 s ago'],
    [300, '5 min ago'],
    [7200, '2 h ago'],
    [172800, '2 d ago'],
  ])('says how long ago after %i seconds', (s, want) => {
    expect(agoText(at, after(s))).toBe(want)
  })

  it('never says a time in the future', () => {
    expect(agoText(at, after(-60))).toBe('just now')
  })

  it('gives nothing for a time that does not parse', () => {
    expect(agoText('soon', after(0))).toBe('')
  })
})
