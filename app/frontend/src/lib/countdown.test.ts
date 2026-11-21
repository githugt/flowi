import { describe, expect, it } from 'vitest'
import { countdown } from './countdown'

const at = '2026-01-01T10:05:00Z'
const base = Date.parse('2026-01-01T10:00:00Z')

describe('countdown', () => {
  it('shows minutes and padded seconds', () => expect(countdown(at, base + 28_000)).toBe('4:32'))
  it('rounds a part of a second up, so it never shows 0:00 before the end', () => expect(countdown(at, base + 299_500)).toBe('0:01'))
  it('says expired at and after the deadline', () => {
    expect(countdown(at, Date.parse(at))).toBe('expired')
    expect(countdown(at, Date.parse(at) + 5000)).toBe('expired')
  })
  it('says nothing without a deadline or with a bad one', () => {
    expect(countdown(undefined, base)).toBe('')
    expect(countdown('soon', base)).toBe('')
  })
})
