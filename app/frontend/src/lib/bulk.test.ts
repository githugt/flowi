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

describe('switched-off rows', () => {
  const off = (Key: string, Value: string) => ({ Key, Value, Description: '', Disabled: true })

  it('should write a switched-off row with a leading # and read it back switched off', () => {
    // Before: a row could not be switched off, so bulk edit had nothing to say about one.
    const rows = [kv('a', '1'), off('debug', '1')]

    const text = rowsToText(rows, true)
    const got = textToRows(text, rows, true)

    expect(text).toBe('a: 1\n# debug: 1')
    expect(got).toEqual(rows)
  })

  it('should read # lines as ordinary names when switching off is not offered', () => {
    const got = textToRows('# note: x', [])

    expect(got).toEqual([kv('# note', 'x')])
  })

  it('should let a hand-typed #name switch a row off and removing the # switch it on', () => {
    const before = [kv('a', '1')]

    const offNow = textToRows('#a: 1', before, true)
    const onAgain = textToRows('a: 1', offNow, true)

    expect(offNow).toEqual([off('a', '1')])
    expect(onAgain).toEqual([kv('a', '1')])
  })
})
