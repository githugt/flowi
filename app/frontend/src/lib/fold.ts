// Searching that ignores case and accents the way a person does: "cafe" finds "Café" and "istanbul" finds "İstanbul".
// Letters that are not an accent on another letter (ß, ø, æ) are left as they are. Text and query are folded alike.
// Before: the filters compared with toLowerCase() alone, so an accented name was missed by its plain spelling.

/** The text in lower case, decomposed, with combining marks removed. */
export function fold(s: string): string {
  // Plain ASCII has nothing to decompose: skip the normalisation, which a search over a big JSON body runs per node.
  if (/^[\x00-\x7f]*$/.test(s)) return s.toLowerCase()
  return s.normalize('NFD').replace(/\p{M}/gu, '').toLocaleLowerCase()
}

/** Whether text contains query, ignoring case and accents. An empty query is in everything. */
export function contains(text: string, query: string): boolean {
  return fold(text).includes(fold(query))
}

/** Whether text starts with query, ignoring case and accents. */
export function startsWith(text: string, query: string): boolean {
  return fold(text).startsWith(fold(query))
}
