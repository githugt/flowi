// What the environment comparison shows: which rows, and what each cell says. The cells come from Go
// (vm.CompareEnvironments); this only decides how to show them.
import type { EnvCompareView, EnvGroup } from './types'

type Row = EnvCompareView['rows'][number]
type Cell = Row['cells'][number]

/** The rows to show: all of them, or only those that differ between environments or have a gap. */
export function visibleRows(rows: Row[], onlyIssues: boolean): Row[] {
  return onlyIssues ? rows.filter((r) => r.differs || r.gaps > 0) : rows
}

const plain = (c: Cell) => c.state === 'set' || c.state === 'inherited'

/** What a plain value is read as: "text" for the default, else its kind. A secret and a missing variable have none. */
export function kindName(c: Cell): string {
  return plain(c) ? c.kind || 'text' : ''
}

/** The type to write in a cell: only in a row where the environments disagree on it, since there it is the difference. */
export function shownKind(row: Row, c: Cell): string {
  return row.kindsDiffer ? kindName(c) : ''
}

// A value that is not read as text says what it is read as, since the same characters differ by type; in a row
// whose environments disagree on it, text says so too.
const asKind = (c: Cell, showText = false) => (c.kind || (showText && plain(c)) ? ` (read as ${kindName(c)})` : '')

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
export function cellLabel(env: string, name: string, c: Cell, kindsDiffer = false): string {
  switch (c.state) {
    case 'secret':
      return `${name} in ${env}: a secret with a value`
    case 'secret-unset':
      return `${name} in ${env}: a secret without a value`
    case 'missing':
      return `${name} in ${env}: not defined`
    case 'inherited':
      return `${name} in ${env}: ${c.value === '' ? 'empty' : c.value}${asKind(c, kindsDiffer)}, from ${c.source}`
    default:
      return `${name} in ${env}: ${c.value === '' ? 'empty' : c.value}${asKind(c, kindsDiffer)}`
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

/**
 * The group the compare screen shows. A group the person chose and that still exists stays. Otherwise the default
 * group when it has at least two environments to compare, else the first named group that has, else the default.
 * Before: the screen listed every environment, so the choice did not exist.
 */
export function pickGroup(groups: EnvGroup[], chosen: string | null): string {
  if (chosen !== null && groups.some((g) => g.name === chosen)) return chosen
  const comparable = groups.filter((g) => g.count >= 2)
  return comparable.find((g) => g.name === '')?.name ?? comparable[0]?.name ?? ''
}
