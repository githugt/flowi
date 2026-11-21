import { describe, expect, it } from 'vitest'
import { childPath, matches, matchPaths, stepMatch, MATCH_LIMIT } from './jsonpath'

describe('childPath', () => {
  it('writes plain members with dots and indexes with brackets', () => {
    expect(childPath(childPath('$', 'data'), 'items')).toBe('$.data.items')
    expect(childPath('$.items', 0)).toBe('$.items[0]')
  })

  it('quotes members that are not identifiers', () => {
    expect(childPath('$', 'x-id')).toBe("$['x-id']")
    expect(childPath('$', "it's")).toBe("$['it\\'s']")
  })
})

describe('matches', () => {
  const body = { user: { name: 'Ann', tags: ['admin'] }, n: 3 }

  it('finds keys and values anywhere below', () => {
    expect(matches(body, '$', 'admin')).toBe(true)
    expect(matches(body, '$', 'NAME')).toBe(true)
    expect(matches(body, '$', 'missing')).toBe(false)
  })

  it('matches everything when the query is empty', () => {
    expect(matches(body, '$', '')).toBe(true)
  })
})

describe('matchPaths', () => {
  const body = { user: { name: 'Ada', tags: ['admin', 'ops'] }, admin: true, note: 'no admin here' }

  it('lists the members that match by name or value, in document order', () => {
    expect(matchPaths(body, 'admin')).toEqual(['$.user.tags[0]', '$.admin', '$.note'])
  })
  it('lists nothing for an empty query', () => {
    expect(matchPaths(body, '')).toEqual([])
  })
  it('stops at the limit', () => {
    const many = Array.from({ length: MATCH_LIMIT + 50 }, () => 'x')
    expect(matchPaths(many, 'x')).toHaveLength(MATCH_LIMIT)
  })
})

describe('stepMatch', () => {
  it('starts at the first match going forward and the last going back', () => {
    expect(stepMatch(-1, 3, false)).toBe(0)
    expect(stepMatch(-1, 3, true)).toBe(2)
  })
  it('wraps round at both ends', () => {
    expect(stepMatch(2, 3, false)).toBe(0)
    expect(stepMatch(0, 3, true)).toBe(2)
  })
  it('has no match to go to when there are none', () => {
    expect(stepMatch(0, 0, false)).toBe(-1)
  })
})
