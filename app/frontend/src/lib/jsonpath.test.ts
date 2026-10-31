import { describe, expect, it } from 'vitest'
import { childPath, matches } from './jsonpath'

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
