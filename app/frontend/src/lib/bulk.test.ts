import { describe, expect, it } from 'vitest'
import { rowsToText, textToRows } from './bulk'

const kv = (Key: string, Value: string, Description = '') => ({ Key, Value, Description })

describe('textToRows', () => {
  it.each([
    { name: 'name: value lines', text: 'a: 1\nb: 2', want: [kv('a', '1'), kv('b', '2')] },
    { name: 'a value holding colons', text: 'url: http://x:8080/a', want: [kv('url', 'http://x:8080/a')] },
    { name: 'a line without a colon', text: 'flag', want: [kv('flag', '')] },
    { name: 'blank lines and a line with no name', text: '\n  \n: orphan\na: 1\n', want: [kv('a', '1')] },
    { name: 'spaces after the colon beyond the first', text: 'a:  two spaces', want: [kv('a', ' two spaces')] },
    { name: 'a template', text: 'Authorization: Bearer {{secret.token}}', want: [kv('Authorization', 'Bearer {{secret.token}}')] },
  ])('should read $name', ({ text, want }) => {
    const got = textToRows(text, [])

    expect(got).toEqual(want)
  })

  it('should keep the description of a row that keeps its name', () => {
    const before = [kv('limit', '10', 'page size'), kv('gone', 'x', 'dropped')]

    const got = textToRows('limit: 20\nnew: 1', before)

    expect(got).toEqual([kv('limit', '20', 'page size'), kv('new', '1')])
  })
})

describe('rowsToText', () => {
  it('should give back the same rows when read again', () => {
    const rows = [kv('a', '1', 'kept'), kv('url', 'http://x:1/y')]

    const got = textToRows(rowsToText(rows), rows)

    expect(got).toEqual(rows)
  })
})
