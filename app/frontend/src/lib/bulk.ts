// Bulk editing of key-value rows (query parameters, headers, form fields, variables …): one
// "name: value" a line, the way Postman and the like write them, so a list can be pasted in whole.
import type { KV } from './types'

/**
 * The rows as text, one "name: value" a line. Descriptions are left out of the text. With `toggle`, a row switched
 * off is written with a leading "# ", the way a commented-out line reads, so it survives editing the text.
 */
export function rowsToText(rows: KV[], toggle = false): string {
  return rows.map((r) => `${toggle && r.Disabled ? '# ' : ''}${r.Key}: ${r.Value}`).join('\n')
}

/**
 * The rows text stands for. A line splits at its first colon, so a value may hold colons (a URL,
 * a time); a line without one is a name with an empty value; blank lines are skipped. A row keeps
 * the description it had under the same name, since the text cannot carry one. With `toggle`, a line that starts
 * with "#" is a row switched off.
 */
export function textToRows(text: string, before: KV[], toggle = false): KV[] {
  const described = new Map(before.map((r) => [r.Key, r.Description ?? '']))
  const out: KV[] = []
  for (let line of text.split('\n')) {
    if (line.trim() === '') continue
    let disabled = false
    if (toggle && line.trimStart().startsWith('#')) {
      disabled = true
      line = line.trimStart().replace(/^#\s?/, '')
    }
    const colon = line.indexOf(':')
    const key = (colon < 0 ? line : line.slice(0, colon)).trim()
    if (key === '') continue
    const value = colon < 0 ? '' : line.slice(colon + 1).replace(/^ /, '')
    out.push({ Key: key, Value: value, Description: described.get(key) ?? '', ...(disabled ? { Disabled: true } : {}) })
  }
  return out
}
