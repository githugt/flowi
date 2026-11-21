// A whole-number field that people type into: "1,000", "1 000", "1_000" and full-width digits are read as written,
// and text that is not a number is an error, never a quiet 0 (where 0 can mean "no limit" or "the default").
// Before: "1,000" became 1 (parseInt) or 0 (Number), and "abc" became 0.

export type CountResult = { value: number; error: string }

/** Reads a count. Empty is 0 (the field's default). Returns an error text and keeps the old value for anything else. */
export function parseCount(text: string): CountResult {
  const folded = text.normalize('NFKC').replace(/[\s  _]/g, '')
  if (folded === '') return { value: 0, error: '' }
  // A comma or a dot is a grouping mark only when every group after the first has exactly three digits.
  const grouped = /^\d{1,3}([,.]\d{3})+$/.test(folded) ? folded.replace(/[,.]/g, '') : folded
  if (/^\d+$/.test(grouped)) {
    const n = Number(grouped)
    if (Number.isSafeInteger(n)) return { value: n, error: '' }
    return { value: 0, error: 'That number is too large.' }
  }
  if (/^-/.test(folded)) return { value: 0, error: 'Write a number of zero or more.' }
  if (/^\d+[.,]\d+$/.test(folded)) return { value: 0, error: 'Write a whole number.' }
  return { value: 0, error: 'Write a whole number, for example 1000.' }
}
