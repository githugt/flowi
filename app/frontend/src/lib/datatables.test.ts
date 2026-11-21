import { describe, expect, it } from 'vitest'
import { isBlankQuery, tableTitle } from './datatables'

describe('the table chips of a data file request', () => {
  it('should describe a table by its file, rows and typed columns', () => {
    // Before: a data file request showed no table, sheet or column names, so the person had to know them to write FROM sales.
    const csv = { name: 'sales', file: 'sales.csv', rows: 3, columns: [{ name: 'id', type: 'INTEGER' }, { name: 'region', type: 'TEXT' }], select: '' }
    const sheet = { name: 'q1', file: 'book.xlsx', sheet: 'Q1', rows: 1, columns: [], select: '' }

    expect(tableTitle(csv)).toBe('sales.csv · 3 rows\nid integer, region text')
    expect(tableTitle(sheet)).toBe('book.xlsx, sheet Q1 · 1 row\n')
  })

  it('should tell a blank statement from a written one', () => {
    expect([isBlankQuery(''), isBlankQuery('  \n'), isBlankQuery('SELECT 1')]).toEqual([true, true, false])
  })
})
