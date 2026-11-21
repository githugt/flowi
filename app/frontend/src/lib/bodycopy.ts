// What "Copy body" and "Copy as CSV" put on the clipboard.
// Before: the response pane could save the body to a file, but not copy it, and a dataset's rows could not leave
// the table.
import type { PartView, TableView } from './types'

type Shown = { bodyJson?: unknown; bodyXml?: string; received: PartView[] }

/** The response body as text: the JSON tree's value indented, the XML as it came, else the body part as shown. */
export function bodyTextOf(r: Shown): string {
  if (r.bodyJson !== undefined && r.bodyJson !== null) return JSON.stringify(r.bodyJson, null, 2)
  if (r.bodyXml) return r.bodyXml
  return r.received.find((p) => p.kind === 'code' && p.heading === 'Body')?.code ?? ''
}

const cell = (v: unknown): string => {
  if (v === null || v === undefined) return ''
  const text = typeof v === 'object' ? JSON.stringify(v) : String(v)
  // A cell that starts like a formula is quoted as text, so a spreadsheet does not run it.
  const safe = /^[=+\-@\t\r]/.test(text) ? "'" + text : text
  return /[",\r\n]/.test(safe) ? '"' + safe.replace(/"/g, '""') + '"' : safe
}

/** The rows of a dataset result as CSV (RFC 4180, CRLF line ends), with the column names first. */
export function tableToCsv(t: Pick<TableView, 'columns' | 'rows'>): string {
  return [t.columns.map(cell).join(','), ...t.rows.map((row) => row.map(cell).join(','))].join('\r\n') + '\r\n'
}
