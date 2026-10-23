/** JSONPath of a member: $.a.b[0]['x-id'] (the form the extract blocks use). */
export function childPath(parent: string, key: string | number): string {
  if (typeof key === 'number') return `${parent}[${key}]`
  return /^[A-Za-z_][A-Za-z0-9_]*$/.test(key) ? `${parent}.${key}` : `${parent}['${key.replace(/'/g, "\\'")}']`
}

/** Whether this value or something under it matches the search text. */
export function matches(value: unknown, key: string, query: string): boolean {
  if (!query) return true
  const q = query.toLowerCase()
  if (key.toLowerCase().includes(q)) return true
  if (value !== null && typeof value === 'object') return Object.entries(value).some(([k, v]) => matches(v, k, query))
  return String(value).toLowerCase().includes(q)
}
