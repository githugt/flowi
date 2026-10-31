// The file keeps durations in nanoseconds; people write 30s.
const UNITS: [string, number][] = [['h', 3.6e12], ['m', 6e10], ['s', 1e9], ['ms', 1e6]]

/** Nanoseconds as the shortest exact form: 30000000000 is "30s". 0 is "". */
export function showDuration(ns: number): string {
  if (!ns) return ''
  for (const [u, n] of UNITS) if (ns % n === 0) return `${ns / n}${u}`
  return `${ns / 1e9}s`
}

/** "500ms", "30s", "2m", "1h" as nanoseconds; "" is 0; anything else is null. */
export function parseDuration(text: string): number | null {
  const t = text.trim()
  if (!t) return 0
  const m = /^(\d+(?:\.\d+)?)(ms|s|m|h)$/.exec(t)
  return m ? Math.round(Number(m[1]) * Object.fromEntries(UNITS)[m[2]]) : null
}
