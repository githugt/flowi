export interface Item {
  insert: string
  detail: string
}

export interface Suggestion {
  from: number
  to: number
  items: Item[]
}

// A word being typed: a name, then dotted members and [0] positions (body.items[0].id). The word
// must not follow a dot or a bracket, which would make it the middle of a longer one.
const word = /(?<![A-Za-z0-9_.\]])[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z0-9_]*|\[\d+\])*$/

/** Whether the caret is inside a quoted text, where names are not read. */
function inText(before: string): boolean {
  let quote = ''
  for (const ch of before) {
    if (quote) quote = ch === quote ? '' : quote
    else if (ch === '"' || ch === "'") quote = ch
  }
  return quote !== ''
}

/**
 * What can complete the word before the caret: the vocabulary entries that start with it. Nothing
 * when the caret is in a quoted text, inside an open {{ (the template completion owns that), or
 * after a space, and nothing for a word that is already complete.
 */
export function suggestFor(text: string, caret: number, vocabulary: Item[], limit = 12): Suggestion | null {
  const before = text.slice(0, caret)
  const open = before.lastIndexOf('{{')
  if ((open >= 0 && !before.slice(open).includes('}}')) || inText(before)) return null
  const m = word.exec(before)
  if (!m) return null
  const typed = m[0].toLowerCase()
  const items = vocabulary.filter((v) => v.insert.toLowerCase().startsWith(typed) && v.insert.toLowerCase() !== typed).slice(0, limit)
  return items.length ? { from: m.index, to: caret, items } : null
}

/** Whether the caret is inside an unclosed {{ ... }}. */
export function inTemplate(text: string, caret: number): boolean {
  const before = text.slice(0, caret)
  const open = before.lastIndexOf('{{')
  return open >= 0 && !before.slice(open).includes('}}')
}

const plain = /^[A-Za-z_][A-Za-z0-9_]*$/
const maxPaths = 200

/**
 * The paths into a parsed body, as an expression writes them: body.user.name, body.items[0].id.
 * A list is offered by its first item, which is how its shape is known. Keys that are not plain
 * words are left out: they are written with ["…"].
 */
export function bodyPaths(value: unknown, prefix: string, detail: string, depth = 4): Item[] {
  const out: Item[] = []
  const walk = (v: unknown, path: string, left: number) => {
    if (out.length >= maxPaths || left === 0 || v === null || typeof v !== 'object') return
    if (Array.isArray(v)) {
      if (v.length === 0) return
      out.push({ insert: `${path}[0]`, detail })
      walk(v[0], `${path}[0]`, left - 1)
      return
    }
    for (const [k, child] of Object.entries(v)) {
      if (!plain.test(k)) continue
      out.push({ insert: `${path}.${k}`, detail })
      walk(child, `${path}.${k}`, left - 1)
    }
  }
  out.push({ insert: prefix, detail })
  walk(value, prefix, depth)
  return out
}
