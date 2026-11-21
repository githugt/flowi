import { describe, expect, it } from 'vitest'
import { checkedText, choicesOf, isStale } from './accessage'

const now = new Date(2026, 10, 20, 12, 0)

describe('checkedText', () => {
  it('Given a check from days ago When shown Then it says how long ago', () => {
    // Before: nothing said when an access check was last run.
    expect(checkedText('2026-11-17 12:00', now)).toBe('Checked 3 days ago')
    expect(checkedText('2026-11-20 12:00', now)).toBe('Checked just now')
  })
})

describe('isStale', () => {
  it('Given a week-old check When asked Then it is stale, and a newer one is not', () => {
    expect(isStale('2026-11-13 12:00', now)).toBe(true)
    expect(isStale('2026-11-14 12:00', now)).toBe(false)
  })
  it('Given a time that cannot be read When asked Then it is not called stale', () => {
    expect(isStale('', now)).toBe(false)
  })
})

describe('choicesOf', () => {
  it('Given a kept check When its form is restored Then the owner and each identity tried are chosen once', () => {
    const v = { owner: 'alice', steps: [{ attempts: [{ identity: 'bob' }, { identity: 'carol' }] }, { attempts: [{ identity: 'bob' }] }] }

    expect(choicesOf(v)).toEqual({ owner: 'alice', others: ['bob', 'carol'] })
  })
})
