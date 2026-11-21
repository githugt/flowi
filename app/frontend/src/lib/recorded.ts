// What the replayer saved while it recorded: a line for the card under the form, and the requests it names.
import { plural } from './plural'

export interface Recorded {
  count: number
  requests: string[]
  target: string
}

/** The card's headline, "Saved 7 new requests in Payments/recorded"; "" when nothing was saved. */
export function recordedHeadline(r: Recorded): string {
  return r.count > 0 ? `Saved ${plural(r.count, 'new request')} in ${r.target}` : ''
}

/** The requests the card lists: the first `limit`, and how many more there are. */
export function recordedList(r: Recorded, limit = 8): { shown: string[]; more: number } {
  return { shown: r.requests.slice(0, limit), more: Math.max(0, r.requests.length - limit) }
}
