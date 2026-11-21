// What a result grid shows: its rows filtered by a word and sorted by a column. The table itself (rows as they came
// from Go) is never changed; a view is a list of the original row numbers, so the # column keeps meaning "row n of the
// result" whatever the order.
import { cellOf } from './cells'

export type Direction = 'asc' | 'desc'
export interface Sort {
  column: number
  direction: Direction
}

/** A click on a column header: ascending, then descending, then the original order. */
export function nextSort(current: Sort | null, column: number): Sort | null {
  if (!current || current.column !== column) return { column, direction: 'asc' }
  return current.direction === 'asc' ? { column, direction: 'desc' } : null
}

function compare(a: unknown, b: unknown): number {
  const x = cellOf(a)
  const y = cellOf(b)
  // NULL goes last whichever way the column is sorted: the caller flips only the order of the rest.
  if (x.kind === 'number' && y.kind === 'number') return (a as number) - (b as number)
  if (x.kind === 'bool' && y.kind === 'bool') return Number(a) - Number(b)
  return x.text.localeCompare(y.text, undefined, { numeric: true, sensitivity: 'base' })
}

/**
 * The row numbers to show, in order: those with a cell containing `query` (any case; empty keeps all), sorted by
 * `sort` (stable; NULLs last in both directions).
 */
export function gridView(rows: unknown[][], query: string, sort: Sort | null): number[] {
  const word = query.trim().toLowerCase()
  const kept: number[] = []
  rows.forEach((row, i) => {
    if (!word || row.some((v) => cellOf(v).text.toLowerCase().includes(word))) kept.push(i)
  })
  if (!sort) return kept
  const sign = sort.direction === 'asc' ? 1 : -1
  return kept.sort((i, j) => {
    const a = rows[i][sort.column]
    const b = rows[j][sort.column]
    const aNull = cellOf(a).kind === 'null'
    const bNull = cellOf(b).kind === 'null'
    if (aNull || bNull) return aNull === bNull ? i - j : aNull ? 1 : -1
    return sign * compare(a, b) || i - j
  })
}

/** The label of a sorted column's header for a screen reader, and its aria-sort value. */
export function ariaSort(sort: Sort | null, column: number): 'ascending' | 'descending' | 'none' {
  if (!sort || sort.column !== column) return 'none'
  return sort.direction === 'asc' ? 'ascending' : 'descending'
}
