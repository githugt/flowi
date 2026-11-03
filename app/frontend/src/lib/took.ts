// The response strip tints its time chip once a send is slow: amber over 500 ms, red over 2 s.
const SLOW_MS = 500
const VERY_SLOW_MS = 2000

export type TookTone = '' | 'slow' | 'verySlow'

/** The tone for a time as Go writes it ("820 µs", "142 ms", "1.24 s", or without the space); "" for a quick or unreadable one. */
export function tookTone(text: string): TookTone {
  const m = /^(\d+(?:\.\d+)?) ?(µs|ms|s)$/.exec(text.trim())
  if (!m) return ''
  const ms = Number(m[1]) * { µs: 0.001, ms: 1, s: 1000 }[m[2] as 'µs' | 'ms' | 's']
  if (ms > VERY_SLOW_MS) return 'verySlow'
  return ms > SLOW_MS ? 'slow' : ''
}
