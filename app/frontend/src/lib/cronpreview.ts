// The line under the cron field of a schedule: the words and the next firings, or the reason it cannot be used.
// Before: a mistake in the cron line or the time zone showed only after Save.
import type { CronPreview } from './types'

/** The text to show and whether it is an error. Nothing when there is nothing to say. */
export function cronPreviewLine(p: CronPreview): { text: string; error: boolean } {
  if (p.error) return { text: p.error, error: true }
  const next = p.next.length ? `Next: ${p.next.join(', ')}` : ''
  const text = [p.words, next].filter(Boolean).join('. ')
  return { text: text && p.words ? text + '.' : text, error: false }
}
