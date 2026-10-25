import { describe, expect, it } from 'vitest'
import { unique } from './vocabulary'

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
