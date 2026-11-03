const MIN = 60_000
const HOUR = 60 * MIN
const DAY = 24 * HOUR

/** How far off a time is, for a label: "in 6 min", "in 4 h 12 min", "in 3 days". "" for a time that is missing, unreadable or past. */
export function untilText(iso: string, now: number): string {
  const ms = Date.parse(iso) - now
  if (!iso || Number.isNaN(ms) || ms < 0) return ''
  if (ms < MIN) return 'in under a minute'
  if (ms > DAY) return `in ${Math.floor(ms / DAY)} days`
  const hours = Math.floor(ms / HOUR)
  const minutes = Math.floor((ms % HOUR) / MIN)
  if (!hours) return `in ${minutes} min`
  return minutes ? `in ${hours} h ${minutes} min` : `in ${hours} h`
}
