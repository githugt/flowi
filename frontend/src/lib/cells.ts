/** A cell of the rows table: what to write, and how to align and colour it. */
export interface Cell {
  text: string
  kind: 'null' | 'number' | 'bool' | 'text'
}

/** NULL is shown as NULL, apart from an empty text; a value that is neither is written as JSON. */
export function cellOf(value: unknown): Cell {
  if (value === null || value === undefined) return { text: 'NULL', kind: 'null' }
  if (typeof value === 'number') return { text: String(value), kind: 'number' }
  if (typeof value === 'boolean') return { text: String(value), kind: 'bool' }
  if (typeof value === 'string') return { text: value, kind: 'text' }
  return { text: JSON.stringify(value), kind: 'text' }
}
