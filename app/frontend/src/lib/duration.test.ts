import { describe, expect, it } from 'vitest'
import { parseDuration, showDuration } from './duration'

describe('durations', () => {
  it('round-trip what people write', () => {
    for (const text of ['500ms', '30s', '2m', '1h']) expect(showDuration(parseDuration(text)!)).toBe(text)
  })

  it('treat empty as none and refuse what is not a duration', () => {
    expect(parseDuration('')).toBe(0)
    expect(showDuration(0)).toBe('')
    expect(parseDuration('soon')).toBeNull()
    expect(parseDuration('30')).toBeNull()
  })
})
