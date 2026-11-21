import { describe, expect, it } from 'vitest'
import { ariaSort, gridView, nextSort } from './gridview'

const rows: unknown[][] = [
  [1, 'pear', 9.5],
  [2, 'Apple', null],
  [3, 'fig', 2],
  [4, null, 30],
]

describe('nextSort', () => {
  it('should go ascending, descending, then back to the original order', () => {
    // Before: the rows grid was a bare table: no sort, no filter.
    const first = nextSort(null, 1)
    const second = nextSort(first, 1)

    expect(first).toEqual({ column: 1, direction: 'asc' })
    expect(second).toEqual({ column: 1, direction: 'desc' })
    expect(nextSort(second, 1)).toBeNull()
  })

  it('should start a new column ascending', () => {
    expect(nextSort({ column: 1, direction: 'desc' }, 2)).toEqual({ column: 2, direction: 'asc' })
  })
})

describe('gridView', () => {
  it('should keep every row in order with no query and no sort', () => {
    expect(gridView(rows, '', null)).toEqual([0, 1, 2, 3])
  })

  it('should sort numbers as numbers, not as text', () => {
    expect(gridView(rows, '', { column: 2, direction: 'asc' })).toEqual([2, 0, 3, 1]) // 2, 9.5, 30, then NULL
    expect(gridView(rows, '', { column: 2, direction: 'desc' })).toEqual([3, 0, 2, 1]) // 30, 9.5, 2, then NULL
  })

  it('should sort text without regard to case and put NULL last either way', () => {
    expect(gridView(rows, '', { column: 1, direction: 'asc' })).toEqual([1, 2, 0, 3])
    expect(gridView(rows, '', { column: 1, direction: 'desc' })).toEqual([0, 2, 1, 3])
  })

  it('should filter on any cell, ignoring case, and keep the original row numbers', () => {
    expect(gridView(rows, 'APPLE', null)).toEqual([1])
    expect(gridView(rows, '30', null)).toEqual([3])
    expect(gridView(rows, 'null', null)).toEqual([1, 3]) // a NULL is written NULL, so the word finds it
    expect(gridView(rows, 'zzz', null)).toEqual([])
  })

  it('should filter first and then sort what is left', () => {
    expect(gridView(rows, 'p', { column: 0, direction: 'desc' })).toEqual([1, 0]) // pear and Apple
  })

  it('should read numbered text in natural order', () => {
    const named: unknown[][] = [['row10'], ['row2'], ['row1']]

    expect(gridView(named, '', { column: 0, direction: 'asc' })).toEqual([2, 1, 0])
  })
})

describe('ariaSort', () => {
  it('should name the sorted column and none other', () => {
    const sort = { column: 1, direction: 'desc' as const }

    expect(ariaSort(sort, 1)).toBe('descending')
    expect(ariaSort(sort, 0)).toBe('none')
    expect(ariaSort(null, 0)).toBe('none')
    expect(ariaSort({ column: 0, direction: 'asc' }, 0)).toBe('ascending')
  })
})

describe('the result grid', () => {
  it('should sort from the header and filter with a box, and number rows by their place in the result', async () => {
    const src = (await import('../ResultGrid.svelte?raw')).default as string

    expect(src).toContain('nextSort(sort, ci)')
    expect(src).toContain('aria-sort={ariaSort(sort, ci)}')
    expect(src).toContain('gridView(table.rows, query, sort)')
    expect(src).toContain('{i + 1}') // i is the original row number, not the position on screen
    expect(src).toContain('label="Filter rows"')
  })
})
