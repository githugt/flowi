/** How an HTTP status reads in a pill: ok (2xx), warn (1xx and 3xx), bad (4xx and 5xx) or none (no status). */
export type StatusTone = 'ok' | 'warn' | 'bad' | ''

/**
 * The tone of a status. `code` may be a number or the text of a pill such as "301 Moved Permanently".
 * `passed` is the fallback for text with no number in it (a message or a query result).
 */
export function statusTone(code: number | string | undefined, passed?: boolean): StatusTone {
  const n = typeof code === 'number' ? code : parseInt(String(code ?? ''), 10)
  // Before: the response pane used "passed" alone, so every kind of status that did not fail looked the same, a 301 included.
  if (Number.isNaN(n) || n <= 0) return passed === undefined ? '' : passed ? 'ok' : 'bad'
  if (n >= 400) return 'bad'
  if (n >= 300 || n < 200) return 'warn'
  return 'ok'
}

/**
 * The text of the response pill. A send whose status is fine but whose checks failed said only "200" in green:
 * the failure showed in the Assert tab, which is not always open. The tone follows the failure too.
 */
export function statusPill(code: number | string | undefined, passed: boolean): { text: string; tone: StatusTone } {
  const tone = statusTone(code, passed)
  if (!passed && tone !== 'bad') return { text: `${code} · check failed`, tone: 'bad' }
  return { text: String(code ?? ''), tone }
}
