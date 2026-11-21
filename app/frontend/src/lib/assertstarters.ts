// Checks offered from the response on screen, so writing one starts from a click instead of an empty box.
// Before: the Checks tab was a blank text row with a help popover; nothing suggested a rule from the response.
import type { ResponseView } from './types'

export interface Starter {
  label: string
  /** The check, as written in the request's assert list. */
  line: string
}

const MAX_FIELDS = 6
const SIMPLE_KEY = /^[A-Za-z_][A-Za-z0-9_]*$/

const literal = (v: unknown): string | null => {
  if (typeof v === 'string') return v.length <= 40 ? JSON.stringify(v) : null // a long text is not a good thing to pin
  if (typeof v === 'number' || typeof v === 'boolean') return String(v)
  return null
}

/** A check that the value at a path is what it is now: `body.items[0].status == "active"`. */
export function valueCheck(path: string, value: unknown): string | null {
  const lit = literal(value)
  if (lit !== null) return `${path} == ${lit}`
  if (Array.isArray(value)) return `len(${path}) == ${value.length}`
  return value === null ? `${path} == empty` : `${path} != empty`
}

/** The checks a response suggests: its status, its time bound, and its top-level JSON fields. Empty with no response. */
export function startersFor(r: Pick<ResponseView, 'code' | 'bodyJson'> | null | undefined): Starter[] {
  if (!r) return []
  const out: Starter[] = []
  const code = Number(r.code)
  if (Number.isInteger(code) && code > 0) {
    out.push({ label: `Status is ${code}`, line: `status == ${code}` })
    const family = Math.floor(code / 100)
    if (family === 2) out.push({ label: 'Status is 2xx', line: 'status >= 200 and status < 300' })
  }
  out.push({ label: 'Responds within 1 s', line: 'response.time < 1s' })
  const json = r.bodyJson
  if (json && typeof json === 'object' && !Array.isArray(json)) {
    for (const [key, value] of Object.entries(json as Record<string, unknown>).slice(0, MAX_FIELDS)) {
      if (!SIMPLE_KEY.test(key)) continue
      const line = valueCheck(`body.${key}`, value)
      if (line) out.push({ label: line, line })
    }
  } else if (Array.isArray(json)) {
    out.push({ label: 'The list is not empty', line: 'len(body) > 0' })
  }
  return out
}
