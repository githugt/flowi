import { describe, expect, it } from 'vitest'
import { startersFor, valueCheck } from './assertstarters'

describe('valueCheck', () => {
  it('pins strings, numbers and booleans, counts lists, and checks the rest exist', () => {
    expect(valueCheck('body.status', 'active')).toBe('body.status == "active"')
    expect(valueCheck('body.n', 3)).toBe('body.n == 3')
    expect(valueCheck('body.ok', true)).toBe('body.ok == true')
    expect(valueCheck('body.items', [1, 2])).toBe('len(body.items) == 2')
    expect(valueCheck('body.owner', { id: 1 })).toBe('body.owner != empty')
    expect(valueCheck('body.gone', null)).toBe('body.gone == empty')
  })
  it('does not pin a long text, only that it is there', () => {
    expect(valueCheck('body.text', 'x'.repeat(41))).toBe('body.text != empty')
  })
})

describe('startersFor', () => {
  it('offers nothing before a response', () => {
    expect(startersFor(null)).toEqual([])
  })
  it('offers the status, the time and the top-level JSON fields', () => {
    // Before: the Checks tab offered no suggestion from the response.
    const lines = startersFor({ code: '200', bodyJson: { id: 7, name: 'Rex', tags: ['a'], 'odd key': 1 } }).map((s) => s.line)

    expect(lines).toEqual(['status == 200', 'status >= 200 and status < 300', 'response.time < 1s', 'body.id == 7', 'body.name == "Rex"', 'len(body.tags) == 1'])
  })
  it('offers a non-empty check for a list body', () => {
    expect(startersFor({ code: '200', bodyJson: [1] }).map((s) => s.line)).toContain('len(body) > 0')
  })
  it('skips the status for a code that is not a number', () => {
    expect(startersFor({ code: '', bodyJson: undefined }).map((s) => s.line)).toEqual(['response.time < 1s'])
  })
})
