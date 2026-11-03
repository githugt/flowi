import { describe, expect, it } from 'vitest'
import { untilText } from './until'

const now = Date.parse('2026-11-02T22:48:00Z')
const at = (minutes: number, seconds = 0) => new Date(now + (minutes * 60 + seconds) * 1000).toISOString()

describe('untilText', () => {
  it.each([
    [at(0, 20), 'in under a minute'],
    [at(6), 'in 6 min'],
    [at(59), 'in 59 min'],
    [at(60), 'in 1 h'],
    [at(252), 'in 4 h 12 min'],
    [at(24 * 60), 'in 24 h'],
    [at(3 * 24 * 60 + 5 * 60), 'in 3 days'],
  ])('%s reads as %s', (iso, text) => {
    expect(untilText(iso, now)).toBe(text)
  })

  it('says nothing for a time that is missing, unreadable or already past', () => {
    expect(untilText('', now)).toBe('')
    expect(untilText('soon', now)).toBe('')
    expect(untilText(at(-5), now)).toBe('')
  })
})
