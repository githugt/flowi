// What the headers table shows: a filter, a copy in the form curl -i prints, and Set-Cookie lines taken apart.
// Before: headers were a collapsed "N rows" block with no filter or copy, and a Set-Cookie line was one long string.
import { contains } from './fold'

export interface HeaderRow {
  key: string
  value: string
}

export interface Cookie {
  name: string
  value: string
  /** Attributes in the order written: "Path=/", "HttpOnly". */
  attrs: string[]
}

/** The rows whose name or value contain the query, ignoring case and accents. An empty query keeps all of them. */
export function filterHeaders(rows: HeaderRow[], query: string): HeaderRow[] {
  const q = query.trim()
  return q ? rows.filter((r) => contains(r.key, q) || contains(r.value, q)) : rows
}

/** The headers as lines of "Name: value", the way curl -i prints them. */
export function headerText(rows: HeaderRow[]): string {
  return rows.map((r) => `${r.key}: ${r.value}`).join('\n')
}

/** Whether the row is a Set-Cookie header. */
export function isSetCookie(row: HeaderRow): boolean {
  return row.key.toLowerCase() === 'set-cookie'
}

/** A Set-Cookie value as a name, a value and its attributes. A value with no "=" has no cookie name: it is returned whole. */
export function parseSetCookie(value: string): Cookie {
  const [first, ...rest] = value.split(';').map((s) => s.trim())
  const eq = first.indexOf('=')
  const cookie = eq < 0 ? { name: '', value: first } : { name: first.slice(0, eq), value: first.slice(eq + 1) }
  return { ...cookie, attrs: rest.filter((a) => a !== '') }
}
