// How old the kept access check is: it is a table of results from one run, and the server or the flow may have changed since.
import { agoText } from './ago'

/** A check older than this many days is marked stale. */
export const STALE_DAYS = 7

/** "Checked 3 days ago" for a check time written as "2006-01-02 15:04" (local time). */
export function checkedText(at: string, now: Date): string {
  return `Checked ${agoText(at, now)}`
}

/** Whether the check is old enough to be marked stale; false when the time cannot be read. */
export function isStale(at: string, now: Date, days = STALE_DAYS): boolean {
  const m = /^(\d{4})-(\d{2})-(\d{2}) (\d{2}):(\d{2})/.exec(at)
  if (!m) return false
  const then = new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]), Number(m[4]), Number(m[5]))
  return now.getTime() - then.getTime() >= days * 24 * 60 * 60 * 1000
}

/** What to preselect when a kept check is shown again: its owner and the identities it tried, so Check runs the same again. */
export function choicesOf(v: { owner: string; steps: { attempts: { identity: string }[] }[] }): { owner: string; others: string[] } {
  const others: string[] = []
  for (const s of v.steps) for (const a of s.attempts) if (!others.includes(a.identity)) others.push(a.identity)
  return { owner: v.owner, others }
}
