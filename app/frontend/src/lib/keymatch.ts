// Whether a key press is a letter shortcut such as Ctrl+S, on any keyboard layout.
// Before: the shortcuts compared e.key with 's' or 'z', so on a Russian, Greek or Hebrew layout (where the key
// reads 'ы' or 'я') Ctrl+S and Ctrl+Z did nothing.

/** The press as the shortcut code sees it. */
export type KeyPress = { key: string; code: string }

/**
 * True when the press is the Latin letter. A layout that types Latin letters (Dvorak, AZERTY) is followed by
 * what the key says; a layout that does not (Cyrillic, Greek, Hebrew…) falls back to the key's place on the board.
 */
export function isLetter(e: KeyPress, letter: string): boolean {
  const key = e.key.toLowerCase()
  if (key === letter) return true
  if (/^[a-z]$/.test(key)) return false // a Latin letter that is another letter: the layout says so
  return e.code === 'Key' + letter.toUpperCase()
}
