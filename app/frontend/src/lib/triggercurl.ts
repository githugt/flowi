// The curl line that starts a flow through the HTTP trigger of `flowi serve`: the route is POST /flows/<flow>/run and
// the token goes in an Authorization header as a bearer token.
// Before: the screen gave a bare token and the route in a sentence; the user had to write the line.

const quote = (s: string) => `'${s.replace(/'/g, `'\\''`)}'`

/** `addr` is where the server listens ("127.0.0.1:8081"); `flow` the flow's name; `token` the trigger token. */
export function triggerCurl(addr: string, flow: string, token: string): string {
  const host = addr.trim() || '127.0.0.1:8081'
  const base = /^https?:\/\//.test(host) ? host : `http://${host}`
  const url = `${base.replace(/\/$/, '')}/flows/${encodeURIComponent(flow || '<flow>')}/run`
  return `curl -X POST -H ${quote('Authorization: Bearer ' + token)} ${quote(url)}`
}
