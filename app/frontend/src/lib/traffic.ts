import type { ReplayerView } from './types'

export type TrafficRow = ReplayerView['traffic'][number]

/** How a status reads: success, redirect, client error or server error. */
export const statusTone = (s: number): 'success' | 'redirect' | 'client' | 'server' =>
  s >= 500 ? 'server' : s >= 400 ? 'client' : s >= 300 ? 'redirect' : 'success'

/** Which calls the log shows: all, the 4xx or the 5xx ones, or the errors the replayer made on purpose. `failed` is 4xx and 5xx. */
export type TrafficMode = 'all' | 'failed' | '4xx' | '5xx' | 'faults'

export const TRAFFIC_MODES: { id: TrafficMode; label: string; title: string }[] = [
  { id: 'all', label: 'All', title: 'Every call' },
  { id: '4xx', label: '4xx', title: 'Calls answered with a client error' },
  { id: '5xx', label: '5xx', title: 'Calls answered with a server error' },
  { id: 'faults', label: 'Injected faults', title: 'Errors the replayer made on purpose' },
]

function inMode(r: TrafficRow, mode: TrafficMode): boolean {
  switch (mode) {
    case 'failed':
      return r.status >= 400
    case '4xx':
      return r.status >= 400 && r.status < 500
    case '5xx':
      return r.status >= 500
    case 'faults':
      return r.match === 'fault'
    default:
      return true
  }
}

/**
 * The calls that answer the log's filter: every typed word must appear in the method, path, status or the request
 * it matched, and the call must be in `mode` (true means `failed`, false `all`). The list itself comes back when
 * nothing narrows it.
 */
export function filterTraffic(rows: TrafficRow[], query: string, mode: TrafficMode | boolean): TrafficRow[] {
  const m: TrafficMode = mode === true ? 'failed' : mode === false ? 'all' : mode
  const words = query.toLowerCase().split(/\s+/).filter(Boolean)
  if (!words.length && m === 'all') return rows
  return rows.filter((r) => {
    if (!inMode(r, m)) return false
    const text = `${r.method} ${r.path} ${r.status} ${r.match} ${r.request} ${r.example}`.toLowerCase()
    return words.every((w) => text.includes(w))
  })
}
