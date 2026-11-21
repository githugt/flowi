import { contains } from './fold'

/** JSONPath of a member: $.a.b[0]['x-id'] (the form the extract blocks use). */
export function childPath(parent: string, key: string | number): string {
  if (typeof key === 'number') return `${parent}[${key}]`
  return /^[A-Za-z_][A-Za-z0-9_]*$/.test(key) ? `${parent}.${key}` : `${parent}['${key.replace(/'/g, "\\'")}']`
}

/** Whether this value or something under it matches the search text. */
export function matches(value: unknown, key: string, query: string): boolean {
  if (!query) return true
  if (contains(key, query)) return true
  if (value !== null && typeof value === 'object') return Object.entries(value).some(([k, v]) => matches(v, k, query))
  return contains(String(value), query)
}

export const MATCH_LIMIT = 1000

/**
 * The paths of the members that match the search themselves (their name, or their value when it is not a
 * container), in document order, at most MATCH_LIMIT. A container is not listed because it holds a match.
 * Before: the search hid the rows that did not match and said nothing about how many were left or where.
 */
export function matchPaths(value: unknown, query: string, path = '$', key = '$', out: string[] = []): string[] {
  if (!query || out.length >= MATCH_LIMIT) return out
  const container = value !== null && typeof value === 'object'
  if (contains(key, query) || (!container && contains(String(value), query))) out.push(path)
  if (container) {
    const isArray = Array.isArray(value)
    for (const [k, v] of Object.entries(value as object)) matchPaths(v, query, childPath(path, isArray ? Number(k) : k), k, out)
  }
  return out
}

/** The index to go to from `from` (-1 for none yet) in a list of `n`, wrapping round at both ends. */
export function stepMatch(from: number, n: number, back: boolean): number {
  if (n <= 0) return -1
  if (from < 0) return back ? n - 1 : 0
  return (from + (back ? -1 : 1) + n) % n
}
