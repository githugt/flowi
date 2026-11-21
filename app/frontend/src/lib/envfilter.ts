// Finding a variable in a long environment.
// Before: the environment editor listed every row with no way to narrow it, so a 40-variable environment was scrolled.
import { contains } from './fold'

/** Rows shown from this many on; fewer need no filter box. */
export const FILTER_FROM = 8

/** Whether a row answers the filter: by name, or by value when it is not a secret. An empty filter matches every row. */
export function envRowMatches(row: { name: string; value: string; secret: boolean }, query: string): boolean {
  const q = query.trim()
  return !q || contains(row.name, q) || (!row.secret && contains(row.value, q))
}
