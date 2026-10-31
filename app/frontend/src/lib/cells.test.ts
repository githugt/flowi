import { describe, expect, it } from 'vitest'
import { cellOf } from './cells'

describe('cellOf', () => {
  it.each([
    { name: 'null', value: null, want: { text: 'NULL', kind: 'null' } },
    { name: 'a missing value', value: undefined, want: { text: 'NULL', kind: 'null' } },
    { name: 'a number', value: 12.5, want: { text: '12.5', kind: 'number' } },
    { name: 'zero', value: 0, want: { text: '0', kind: 'number' } },
    { name: 'true', value: true, want: { text: 'true', kind: 'bool' } },
    { name: 'false', value: false, want: { text: 'false', kind: 'bool' } },
    { name: 'text', value: 'Ada', want: { text: 'Ada', kind: 'text' } },
    { name: 'empty text, which is not NULL', value: '', want: { text: '', kind: 'text' } },
    { name: 'a JSON value', value: { a: [1, 2] }, want: { text: '{"a":[1,2]}', kind: 'text' } },
  ])('should show $name', ({ value, want }) => {
    const got = cellOf(value)

    expect(got).toEqual(want)
  })
})
