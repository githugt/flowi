// How long a toast stays: long enough to read it. A short note keeps 4 seconds and an error 8, and every character
// past 60 adds 50 ms, up to 20 seconds.
// Before: a help text of two sentences left after 4 seconds, as fast as "Copied".
export function toastLife(text: string, level: string): number {
  const base = level === 'error' ? 8000 : 4000
  const extra = Math.max(0, text.length - 60) * 50
  return Math.min(base + extra, 20000)
}
