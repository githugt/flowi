import { describe, expect, it } from 'vitest'
import { envRowMatches } from './envfilter'

describe('envRowMatches', () => {
  const row = (name: string, value: string, secret = false) => ({ name, value, secret })

  it('matches every row for an empty filter', () => {
    expect(envRowMatches(row('a', 'b'), '')).toBe(true)
    expect(envRowMatches(row('a', 'b'), '  ')).toBe(true)
  })
  it('looks in the name, and in the value of a plain row', () => {
    // Before: no filter existed.
    expect(envRowMatches(row('API_BASE', 'https://x.test'), 'base')).toBe(true)
    expect(envRowMatches(row('API_BASE', 'https://x.test'), 'x.test')).toBe(true)
    expect(envRowMatches(row('API_BASE', 'https://x.test'), 'zzz')).toBe(false)
  })
  it('never looks in a secret value', () => {
    expect(envRowMatches(row('TOKEN', 'hunter2', true), 'hunter')).toBe(false)
    expect(envRowMatches(row('TOKEN', 'hunter2', true), 'token')).toBe(true)
  })
})
