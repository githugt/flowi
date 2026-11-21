// Checks a JSON body as it is written, as the XML body is checked: the first problem and where it is.
// Before: only an XML body said whether it read; a JSON body with a missing comma showed nothing until the server
// answered 400.

export type JsonProblem = { line: number; column: number; message: string }

// A template outside a string stands for a value: it is blanked to a number of the same width, so the positions
// the parser reports still match the text. A template inside a string is part of the string and needs no care.
function blankTemplates(text: string): string {
  let out = ''
  let inString = false
  for (let i = 0; i < text.length; i++) {
    const c = text[i]
    if (inString) {
      out += c
      if (c === '\\') out += text[++i] ?? ''
      else if (c === '"') inString = false
    } else if (c === '"') {
      inString = true
      out += c
    } else if (c === '{' && text[i + 1] === '{' && text.indexOf('}}', i) >= 0) {
      const end = text.indexOf('}}', i) + 2
      out += '0'.padEnd(end - i, ' ')
      i = end - 1
    } else {
      out += c
    }
  }
  return out
}

/** The first problem of a JSON text, or null when it reads (or is empty). */
export function jsonProblem(text: string): JsonProblem | null {
  if (!text.trim()) return null
  const source = blankTemplates(text)
  try {
    JSON.parse(source)
    return null
  } catch (e) {
    const raw = e instanceof Error ? e.message : String(e)
    const at = /position (\d+)/.exec(raw)
    const offset = at ? Math.min(Number(at[1]), source.length) : source.length
    const before = source.slice(0, offset).split('\n')
    const message = raw.replace(/\s*(in JSON )?at position \d+.*$/s, '').replace(/^JSON\.parse: /, '')
    return { line: before.length, column: before[before.length - 1].length + 1, message }
  }
}
