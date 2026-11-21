// How long a send has been running, for the Cancel button: "0.4 s", "12 s", "1 m 05 s".
// Before: Send turned into Cancel with nothing else changing, so a slow request looked the same as one that had
// just started.
export function elapsedText(ms: number): string {
  const s = Math.max(0, ms) / 1000
  if (s < 10) return `${s.toFixed(1)} s`
  if (s < 60) return `${Math.floor(s)} s`
  const m = Math.floor(s / 60)
  return `${m} m ${String(Math.floor(s % 60)).padStart(2, '0')} s`
}
