// "Copy as curl" for a call the replayer answered: the line that makes the same call again.
// Before: the traffic log showed a call's headers and body but gave no way to repeat it.

const quote = (s: string) => `'${s.replace(/'/g, `'\\''`)}'`

// Sent by curl itself or by the connection, not part of what the caller chose.
const SKIPPED = new Set(['host', 'content-length', 'connection', 'accept-encoding', 'user-agent'])

export interface CurlCall {
  method: string
  path: string
}
export interface CurlPart {
  headers: { name: string; value: string }[]
  body: string
  truncated: boolean
}

/**
 * The curl line for `call` sent to the replayer at `addr` ("127.0.0.1:8080"). `sent` is what the caller sent, when the
 * replayer kept it. A body that was cut short is left out, as half a body would make a different call.
 */
export function callCurl(addr: string, call: CurlCall, sent?: CurlPart): string {
  const base = /^https?:\/\//.test(addr) ? addr : `http://${addr.trim() || '127.0.0.1:8080'}`
  const parts = ['curl', '-X', call.method, quote(base.replace(/\/$/, '') + call.path)]
  for (const h of sent?.headers ?? []) {
    if (!SKIPPED.has(h.name.toLowerCase())) parts.push('-H', quote(`${h.name}: ${h.value}`))
  }
  if (sent?.body && !sent.truncated) parts.push('--data-raw', quote(sent.body))
  return parts.join(' ')
}
