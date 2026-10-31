import { describe, expect, it } from 'vitest'
import { responsePaths, unique } from './vocabulary'
import type { ResponseView } from './types'

describe('unique', () => {
  it('keeps the first of a repeated name, so the live response wins over the saved one', () => {
    const got = unique([
      { insert: 'body.id', detail: 'from the last response' },
      { insert: 'body.name', detail: 'from the last response' },
      { insert: 'body.id', detail: 'from example' },
    ])

    expect(got).toEqual([
      { insert: 'body.id', detail: 'from the last response' },
      { insert: 'body.name', detail: 'from the last response' },
    ])
  })
})

describe('responsePaths of a dataset response', () => {
  const response = {
    table: { columns: ['id', 'name'], rows: [[1, 'Ada']], rowCount: 1, truncated: false },
    messages: [],
  } as unknown as ResponseView

  it('offers the columns of the first row, the way a check reads them', () => {
    const got = responsePaths(response, 'dataset', true).map((i) => i.insert)

    expect(got).toContain('rows[0].id')
    expect(got).toContain('rows[0].name')
  })

  it('writes a column that is not a plain name with brackets', () => {
    const odd = { table: { columns: ['Full Name', 'order-id', 'a"b'], rows: [], rowCount: 0, truncated: false }, messages: [] } as unknown as ResponseView

    const got = responsePaths(odd, 'dataset', true).map((i) => i.insert)

    expect(got).toEqual(['rows[0]["Full Name"]', 'rows[0]["order-id"]', 'rows[0]["a\\"b"]'])
  })

  it('offers nothing before the query has run', () => {
    expect(responsePaths(undefined, 'dataset', true)).toEqual([])
  })

  it('offers nothing for a query that returned no table', () => {
    expect(responsePaths({ messages: [] } as unknown as ResponseView, 'dataset', true)).toEqual([])
  })
})
