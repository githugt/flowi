// Header names and values the editor suggests, and the headers Flowi adds itself, shown dimmed under the table.
// Before: nothing suggested a header, and Content-Type from the Body was invisible until the request went out.

export const COMMON_HEADERS = [
  'Accept', 'Accept-Language', 'Authorization', 'Cache-Control', 'Content-Type', 'Cookie', 'If-Match', 'If-None-Match',
  'If-Modified-Since', 'Idempotency-Key', 'Origin', 'Referer', 'User-Agent', 'X-Request-Id', 'X-Forwarded-For', 'X-API-Key',
]

const CONTENT_TYPES = ['application/json', 'application/xml', 'application/x-www-form-urlencoded', 'multipart/form-data', 'text/plain', 'text/html', 'application/octet-stream']

const VALUES: Record<string, string[]> = {
  'content-type': CONTENT_TYPES,
  accept: ['application/json', 'application/xml', 'text/html', 'text/plain', '*/*'],
  'cache-control': ['no-cache', 'no-store', 'max-age=0'],
  'accept-language': ['en', 'en-US', 'de', 'fr'],
}

/** The values suggested for header `name` (any case); none for a free-text header. */
export function headerValues(name: string): string[] {
  return VALUES[name.trim().toLowerCase()] ?? []
}

export interface ImplicitHeader {
  name: string
  value: string
  /** Where it comes from: "from Body", "from Auth". */
  source: string
}

interface Shape {
  body: { JSON?: string; Text?: string; Form?: unknown[] | null; Multipart?: unknown[] | null; File?: string }
  /** The names of the headers the request sets, as typed. */
  headers: string[]
  /** An auth scheme is chosen. */
  hasAuth: boolean
}

/**
 * The headers Flowi adds that the request does not set itself: the Content-Type of the body (a Content-Type the
 * request sets wins, so none is listed then) and an Authorization header when an auth scheme is chosen.
 */
export function implicitHeaders(s: Shape): ImplicitHeader[] {
  const set = new Set(s.headers.map((h) => h.trim().toLowerCase()))
  const out: ImplicitHeader[] = []
  const b = s.body
  const type = b.JSON ? 'application/json' : b.Text ? 'text/plain; charset=utf-8' : b.Form?.length ? 'application/x-www-form-urlencoded' : b.Multipart?.length ? 'multipart/form-data' : ''
  if (type && !set.has('content-type')) out.push({ name: 'Content-Type', value: type, source: 'from Body' })
  if (s.hasAuth && !set.has('authorization')) out.push({ name: 'Authorization', value: '…', source: 'from Auth' })
  return out
}
