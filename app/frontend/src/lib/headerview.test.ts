import { describe, expect, it } from 'vitest'
import { filterHeaders, headerText, isSetCookie, parseSetCookie } from './headerview'

const rows = [
  { key: 'Content-Type', value: 'application/json' },
  { key: 'Set-Cookie', value: 'sid=abc123; Path=/; HttpOnly; SameSite=Lax' },
  { key: 'X-RateLimit-Remaining', value: '41' },
]

describe('filterHeaders', () => {
  it('matches the name or the value, ignoring case', () => {
    expect(filterHeaders(rows, 'RATELIMIT').map((r) => r.key)).toEqual(['X-RateLimit-Remaining'])
    expect(filterHeaders(rows, 'json').map((r) => r.key)).toEqual(['Content-Type'])
  })
  it('keeps everything for an empty or blank query', () => {
    expect(filterHeaders(rows, '  ')).toHaveLength(3)
  })
})

describe('headerText', () => {
  it('writes Name: value lines', () => {
    expect(headerText(rows.slice(0, 1))).toBe('Content-Type: application/json')
  })
})

describe('parseSetCookie', () => {
  it('splits the cookie from its attributes', () => {
    expect(isSetCookie(rows[1])).toBe(true)
    expect(parseSetCookie(rows[1].value)).toEqual({ name: 'sid', value: 'abc123', attrs: ['Path=/', 'HttpOnly', 'SameSite=Lax'] })
  })
  it('keeps an = inside the value', () => {
    expect(parseSetCookie('t=a=b==').value).toBe('a=b==')
  })
  it('has no name when there is no =', () => {
    expect(parseSetCookie('junk')).toEqual({ name: '', value: 'junk', attrs: [] })
  })
})
