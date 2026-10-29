// Text pasted into a request's address that is really something to import: a curl command, a .http
// request, an API description or a collection. recognise says which, so the window can offer to fill
// the request from it or import it, instead of pasting it as an address.

export interface PastedKind {
  /** The import kind (flowi.ImportKind). */
  id: string
  /** What the offer calls it. */
  label: string
  /** Its first request can fill the open request (Go's FillFromPaste). */
  fills: boolean
}

interface Rule extends PastedKind {
  matches: (text: string, json: Record<string, any> | null) => boolean
}

const METHOD_LINE = /^(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS|QUERY)\s+(https?:\/\/|\{\{)\S*(\s+HTTP\/[\d.]+)?\s*$/m
const yamlKey = (key: string) => new RegExp(`^${key}\\s*:`, 'm')

// In order: the first rule that matches wins. A plain address (https://…) matches none, so it pastes as before.
const rules: Rule[] = [
  { id: 'curl', label: 'a curl command', fills: true, matches: (t) => /^(\$\s*)?curl\s/i.test(t) },
  { id: 'http', label: 'a .http request', fills: true, matches: (t) => /\n/.test(t) && METHOD_LINE.test(t.split('\n').find((l) => l.trim() && !l.startsWith('#')) ?? '') },
  { id: 'openapi', label: 'an OpenAPI document', fills: false, matches: (t, j) => (j ? 'openapi' in j || 'swagger' in j : yamlKey('(openapi|swagger)').test(t)) },
  { id: 'asyncapi', label: 'an AsyncAPI document', fills: false, matches: (t, j) => (j ? 'asyncapi' in j : yamlKey('asyncapi').test(t)) },
  { id: 'har', label: 'a HAR file', fills: false, matches: (_, j) => Array.isArray(j?.log?.entries) },
  {
    id: 'postman',
    label: 'a Postman collection',
    fills: false,
    matches: (_, j) => !!j?.info && (!!j.info._postman_id || String(j.info.schema ?? '').includes('getpostman')),
  },
]

/** The kind of the pasted text, or null when it is not one the window offers to import. */
export function recognise(text: string): PastedKind | null {
  const t = text.trim()
  if (!t) return null
  const json = t.startsWith('{') ? parseObject(t) : null
  const rule = rules.find((r) => r.matches(t, json))
  return rule ? { id: rule.id, label: rule.label, fills: rule.fills } : null
}

function parseObject(text: string): Record<string, any> | null {
  try {
    const v = JSON.parse(text)
    return v && typeof v === 'object' && !Array.isArray(v) ? v : null
  } catch {
    return null // not JSON after all: the YAML rules still look at it
  }
}
