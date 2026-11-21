import { contains } from './fold'

/** The most message rows the live feed keeps on screen. */
export const FEED_MAX_ROWS = 5000

/**
 * How many of the oldest rows are left out so at most `max` show. A feed that adds rows for ever would make
 * the window slower with every message.
 */
export function droppedRows(total: number, max = FEED_MAX_ROWS): number {
  return Math.max(0, total - max)
}

/** Whether the list is scrolled to its end (within a few pixels): a new row then keeps the end in view. */
export function atEnd(scrollTop: number, clientHeight: number, scrollHeight: number, slack = 24): boolean {
  return scrollHeight - (scrollTop + clientHeight) <= slack
}

/** The messages as one JSON object per line, for a file or the clipboard. */
export function toNdjson(messages: readonly { direction: string; at: string; body: string; key?: string; kind?: string; type?: string }[]): string {
  return messages.map((m) => JSON.stringify(m)).join('\n')
}

/**
 * Whether a message answers the filter text: in its body, key or type, ignoring case and accents. An empty filter matches all.
 * Before: the feed lowercased the body, key and type of every message again for each keystroke, three times over.
 */
export function messageMatches(m: { body: string; key?: string; type?: string }, query: string): boolean {
  return !query || contains(m.body, query) || contains(m.key ?? '', query) || contains(m.type ?? '', query)
}

/** Whether the list is a new run: it has become shorter than it was, so what was open, frozen or cleared belongs to the last one. */
export function isNewRun(before: number, now: number): boolean {
  return now < before
}
