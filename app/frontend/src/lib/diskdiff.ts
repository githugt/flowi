// The "Compare" under the banner "This file changed on disk": what differs between the file and the edits, as rows.
// Before: the banner offered Load from disk or Keep mine and nothing to choose by.

export interface DiskChange {
  path: string
  kind: string
  disk: string
  mine: string
}

export interface DiskDiff {
  changes: DiskChange[]
  more: number
  hidden: number
}

/** "$.Versions[0].URL" → "Versions[0].URL": the field as the file's own model names it, without the root marker. */
export function fieldName(path: string): string {
  return path.replace(/^\$\.?/, '') || 'the whole file'
}

/** A value as shown in a cell: "" is "nothing", so an added or removed field reads as such. */
export function cell(value: string): string {
  return value === '' ? '(nothing)' : value
}

/** One line over the rows: how many fields differ, what was not listed and what was left out. */
export function diffSummary(d: DiskDiff): string {
  const n = d.changes.length + d.more
  if (n === 0 && d.hidden === 0) return 'Nothing differs: the file only changed in its layout or comments.'
  const parts: string[] = []
  if (n > 0) parts.push(`${n} ${n === 1 ? 'field differs' : 'fields differ'}`)
  else parts.push('No field you can see differs')
  if (d.more > 0) parts.push(`${d.more} not listed`)
  if (d.hidden > 0) parts.push(`${d.hidden} secret ${d.hidden === 1 ? 'value' : 'values'} left out, so a change there is not shown`)
  return parts.join('; ') + '.'
}
