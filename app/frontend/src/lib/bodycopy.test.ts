import { describe, expect, it } from 'vitest'
import { bodyTextOf, tableToCsv } from './bodycopy'

describe('bodyTextOf', () => {
  it('indents a JSON body', () => {
    expect(bodyTextOf({ bodyJson: { a: 1 }, received: [] })).toBe('{\n  "a": 1\n}')
  })
  it('keeps an XML body as it came', () => {
    expect(bodyTextOf({ bodyXml: '<a/>', received: [] })).toBe('<a/>')
  })
  it('falls back to the body part', () => {
    expect(bodyTextOf({ received: [{ heading: 'Body', kind: 'code', code: 'hello' }] })).toBe('hello')
  })
  it('gives "" for a response with no body', () => {
    expect(bodyTextOf({ received: [{ heading: 'Headers', kind: 'table' }] })).toBe('')
  })
})

describe('tableToCsv', () => {
  it('writes the columns, then each row, with CRLF', () => {
    expect(tableToCsv({ columns: ['id', 'name'], rows: [[1, 'Ada']] })).toBe('id,name\r\n1,Ada\r\n')
  })
  it('quotes commas, quotes and line breaks', () => {
    expect(tableToCsv({ columns: ['n'], rows: [['a,b'], ['say "hi"'], ['x\ny']] })).toBe('n\r\n"a,b"\r\n"say ""hi"""\r\n"x\ny"\r\n')
  })
  it('writes null as empty and objects as JSON', () => {
    expect(tableToCsv({ columns: ['a', 'b'], rows: [[null, { k: 1 }]] })).toBe('a,b\r\n,"{""k"":1}"\r\n')
  })
  it('defuses a cell a spreadsheet would run as a formula', () => {
    expect(tableToCsv({ columns: ['f'], rows: [['=SUM(A1)']] })).toBe("f\r\n'=SUM(A1)\r\n")
  })
})
