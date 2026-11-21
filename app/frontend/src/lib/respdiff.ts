// How the pane words a comparison of two responses (RequestService.ComparePrevious).
// Before: the pane could not say how a response differed from the one before it.

export interface Change {
  path: string
  kind: string
  old: string
  new: string
}

export interface ResponseDiff {
  older: string
  newer: string
  same: boolean
  oldStatus: number
  newStatus: number
  headers: Change[]
  body: Change[]
  bodyMode: string
  truncated: number
  hidden: number
  volatile: number
}

const count = (n: number, one: string, many = one + 's') => `${n} ${n === 1 ? one : many}`

/** The one line above the list: "Status 200 → 404 · 2 headers · 3 body changes". */
export function diffSummary(d: ResponseDiff): string {
  if (d.same) return 'Nothing differs: same status, same headers, same body.'
  const parts: string[] = []
  if (d.oldStatus !== d.newStatus) parts.push(`Status ${d.oldStatus} → ${d.newStatus}`)
  if (d.headers.length) parts.push(count(d.headers.length, 'header'))
  const bodyN = d.body.length + d.truncated
  if (bodyN) parts.push(count(bodyN, d.bodyMode === 'text' ? 'line change' : 'body change'))
  return parts.join(' · ')
}

/** The notes under the list: what was left out of the comparison and why. */
export function diffNotes(d: ResponseDiff): string[] {
  const notes: string[] = []
  if (d.truncated) notes.push(`${count(d.truncated, 'more change')} not listed.`)
  if (d.hidden) notes.push(`${count(d.hidden, 'value')} named like a credential ${d.hidden === 1 ? 'was' : 'were'} not compared.`)
  if (d.volatile) notes.push(d.volatile === 1 ? '1 header that changes on every send (Date, a request id) was skipped.' : `${d.volatile} headers that change on every send (Date, a request id) were skipped.`)
  if (d.bodyMode === 'binary') notes.push('The body is binary: compared by size and digest only.')
  return notes
}

/** The mark before a change: + added, − removed, ~ changed. */
export const kindMark = (kind: string): string => (kind === 'added' ? '+' : kind === 'removed' ? '−' : '~')
