/** Which tree groups the person folded: kept for the next start, and across rebuilds of the tree. */
const KEY = 'flowi.tree.folded'

/** The folded ids after a group is opened or folded. */
export function toggleFolded(folded: string[], id: string): string[] {
  return folded.includes(id) ? folded.filter((f) => f !== id) : [...folded, id]
}

/** Reads the stored list; anything that is not a list of text is dropped. */
export function parseFolded(raw: string | null): string[] {
  if (!raw) return []
  try {
    const v: unknown = JSON.parse(raw)
    return Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : []
  } catch {
    return []
  }
}

export function loadFolded(): string[] {
  try {
    return parseFolded(localStorage.getItem(KEY))
  } catch {
    return []
  }
}

export function saveFolded(folded: string[]) {
  try {
    localStorage.setItem(KEY, JSON.stringify(folded))
  } catch {
    // storage may be blocked: the groups then stay folded for this session only
  }
}
