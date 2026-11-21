// What the environment comparison shows: which rows, and what each cell says. The cells come from Go
// (vm.CompareEnvironments); this only decides how to show them.
import type { EnvCompareView } from './types'

type Row = EnvCompareView['rows'][number]
type Cell = Row['cells'][number]

/** The rows to show: all of them, or only those that differ between environments or have a gap. */
export function visibleRows(rows: Row[], onlyIssues: boolean): Row[] {
  return onlyIssues ? rows.filter((r) => r.differs || r.gaps > 0) : rows
}

// A value that is not read as text says what it is read as, since the same characters differ by type.
const asKind = (c: Cell) => (c.kind ? ` (read as ${c.kind})` : '')

/** The text in a cell. A secret is never shown, only whether it holds a value. */
export function cellText(c: Cell): string {
  switch (c.state) {
    case 'secret':
      return '● set'
    case 'secret-unset':
      return '○ no value'
    case 'missing':
      return '—'
    default:
      return c.value === '' ? '(empty)' : c.value
  }
}

/** The words for a screen reader and the tooltip: what the cell is, beyond its text. */
export function cellLabel(env: string, name: string, c: Cell): string {
  switch (c.state) {
    case 'secret':
      return `${name} in ${env}: a secret with a value`
    case 'secret-unset':
      return `${name} in ${env}: a secret without a value`
    case 'missing':
      return `${name} in ${env}: not defined`
    case 'inherited':
      return `${name} in ${env}: ${c.value === '' ? 'empty' : c.value}${asKind(c)}, from ${c.source}`
    default:
      return `${name} in ${env}: ${c.value === '' ? 'empty' : c.value}${asKind(c)}`
  }
}

/** A line for the foot of the table: how many variables differ and how many have a gap. */
export function summary(rows: Row[]): string {
  const differ = rows.filter((r) => r.differs).length
  const gaps = rows.filter((r) => r.gaps > 0).length
  if (!rows.length) return 'No variables.'
  if (!differ && !gaps) return `${rows.length} variables, the same everywhere.`
  const parts = []
  if (differ) parts.push(`${differ} ${differ === 1 ? 'differs' : 'differ'}`)
  if (gaps) parts.push(`${gaps} ${gaps === 1 ? 'has a gap' : 'have gaps'}`)
  return `${rows.length} variables: ${parts.join(', ')}.`
}
