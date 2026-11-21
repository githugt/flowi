// "moved 12 days ago" for a time the window got as "2026-11-07 10:00" (local time, as Go wrote it).
// Before: the trash showed only the clock time, so what had sat there for a month looked like what had just arrived.

const MINUTE = 60_000
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR

/** How long before `now` the time `at` was, in words; the text itself when it cannot be read. */
export function agoText(at: string, now: Date): string {
  const m = /^(\d{4})-(\d{2})-(\d{2}) (\d{2}):(\d{2})/.exec(at)
  if (!m) return at
  const then = new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]), Number(m[4]), Number(m[5]))
  const ms = now.getTime() - then.getTime()
  if (Number.isNaN(ms) || ms < 0) return at
  const plural = (n: number, unit: string) => `${n} ${unit}${n === 1 ? '' : 's'} ago`
  if (ms < MINUTE) return 'just now'
  if (ms < HOUR) return plural(Math.floor(ms / MINUTE), 'minute')
  if (ms < DAY) return plural(Math.floor(ms / HOUR), 'hour')
  if (ms < 2 * DAY) return 'yesterday'
  if (ms < 60 * DAY) return plural(Math.floor(ms / DAY), 'day')
  return at
}
