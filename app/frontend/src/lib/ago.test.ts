import { describe, expect, it } from 'vitest'
import { agoText } from './ago'

const now = new Date(2026, 10, 19, 12, 0) // 19 November 2026, 12:00

describe('agoText', () => {
  it('counts minutes, hours and days', () => {
    // Before: the trash showed the clock time only, so a month-old draft looked like a new one.
    expect(agoText('2026-11-19 11:55', now)).toBe('5 minutes ago')
    expect(agoText('2026-11-19 11:59', now)).toBe('1 minute ago')
    expect(agoText('2026-11-19 09:00', now)).toBe('3 hours ago')
    expect(agoText('2026-11-07 12:00', now)).toBe('12 days ago')
  })
  it('says just now and yesterday', () => {
    expect(agoText('2026-11-19 12:00', now)).toBe('just now')
    expect(agoText('2026-11-18 11:00', now)).toBe('yesterday')
  })
  it('gives the date itself after two months, for a time in the future, or text it cannot read', () => {
    expect(agoText('2026-08-01 08:00', now)).toBe('2026-08-01 08:00')
    expect(agoText('2026-11-20 08:00', now)).toBe('2026-11-20 08:00')
    expect(agoText('later', now)).toBe('later')
  })
})
