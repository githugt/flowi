// Compares the field names of the hand-written interfaces in types.ts with the classes generated from Go.
// Before: a Go change could rename or drop a field and the UI kept reading it, with no build error.

/** Field names of every generated class, by class name (this["name"] = ... in models.js). */
export function bindingFields(models: string): Map<string, Set<string>> {
  const out = new Map<string, Set<string>>()
  let current: Set<string> | undefined
  for (const line of models.split('\n')) {
    const cls = /^export class (\w+)\b/.exec(line)
    if (cls) {
      current = new Set()
      out.set(cls[1], current)
      continue
    }
    const field = /^\s*this\["(\w+)"\]\s*=/.exec(line)
    if (field && current) current.add(field[1])
  }
  return out
}

/** Field names of every hand-written interface, by interface name (top-level fields only). */
export function interfaceFields(types: string): Map<string, Set<string>> {
  const out = new Map<string, Set<string>>()
  const lines = types.split('\n')
  for (let i = 0; i < lines.length; i++) {
    const head = /^export interface (\w+)(?:<[^>]*>)?\s*(?:extends [^{]*)?\{\s*$/.exec(lines[i])
    if (!head) continue
    const fields = new Set<string>()
    for (i++; i < lines.length && !/^\}/.test(lines[i]); i++) {
      const field = /^ {2}(?:readonly )?(\w+)\??\s*:/.exec(lines[i])
      if (field) fields.add(field[1])
    }
    out.set(head[1], fields)
  }
  return out
}

/** For each interface that has a generated class of the same name: its fields the class does not have. */
export function drift(types: Map<string, Set<string>>, models: Map<string, Set<string>>): Record<string, string[]> {
  const out: Record<string, string[]> = {}
  for (const [name, fields] of types) {
    const generated = models.get(name)
    if (!generated) continue
    const missing = [...fields].filter((f) => !generated.has(f)).sort()
    if (missing.length) out[name] = missing
  }
  return out
}
