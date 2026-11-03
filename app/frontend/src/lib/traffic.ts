import type { ReplayerView } from './types'

export type TrafficRow = ReplayerView['traffic'][number]

/** How a status reads: success, redirect, client error or server error. */
export const statusTone = (s: number): 'success' | 'redirect' | 'client' | 'server' =>
  s >= 500 ? 'server' : s >= 400 ? 'client' : s >= 300 ? 'redirect' : 'success'

/**
 * The calls that answer the log's filter: every typed word must appear in the method, path, status or the request
 * it matched; `failedOnly` keeps the 4xx and 5xx ones. The list itself comes back when nothing narrows it.
 */
export function filterTraffic(rows: TrafficRow[], query: string, failedOnly: boolean): TrafficRow[] {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean)
  if (!words.length && !failedOnly) return rows
  return rows.filter((r) => {
    if (failedOnly && r.status < 400) return false
    const text = `${r.method} ${r.path} ${r.status} ${r.match} ${r.request} ${r.example}`.toLowerCase()
    return words.every((w) => text.includes(w))
  })
}
