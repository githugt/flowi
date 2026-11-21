// What the Extract tab says under a row: the value the last send took, or that the path matched nothing.
// Before: the row was two bare inputs, and a wrong path showed only when a later step read nothing.

export interface ExtractedItem {
  name: string
  value: string
  found: boolean
}

export interface ExtractChip {
  state: 'ok' | 'none'
  text: string
}

/**
 * The chip for the extraction called `name`, or null when the last send did not try it (none yet, or the row was
 * added or renamed after it): no chip is better than a green one that was never checked.
 */
export function extractChip(name: string, response: { extracted: ExtractedItem[] } | undefined | null): ExtractChip | null {
  const item = name.trim() && response ? response.extracted.find((e) => e.name === name) : undefined
  if (!item) return null
  if (!item.found) return { state: 'none', text: 'no match' }
  return { state: 'ok', text: item.value === '' ? '(empty)' : item.value }
}
