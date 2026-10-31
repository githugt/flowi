// Characters that cannot be seen but change what is sent: a zero-width space
// pasted with a URL, a non-breaking space, a byte-order mark, a tab, a space at
// the end of a token. The fields show a button when their text has any.

export interface Hidden {
  index: number
  /** U+200B */
  code: string
  /** "zero-width space" */
  name: string
}

const NAMED: Record<number, string> = {
  0x00a0: 'non-breaking space',
  0x00ad: 'soft hyphen',
  0x180e: 'Mongolian vowel separator',
  0x200b: 'zero-width space',
  0x200c: 'zero-width non-joiner',
  0x200d: 'zero-width joiner',
  0x200e: 'left-to-right mark',
  0x200f: 'right-to-left mark',
  0x2028: 'line separator',
  0x2029: 'paragraph separator',
  0x202f: 'narrow no-break space',
  0x2060: 'word joiner',
  0xfeff: 'byte-order mark',
}

const isInvisible = (cp: number) =>
  cp in NAMED ||
  (cp >= 0x202a && cp <= 0x202e) || // bidirectional overrides
  (cp >= 0x2061 && cp <= 0x2064) ||
  (cp >= 0x2066 && cp <= 0x206f) ||
  (cp >= 0x2000 && cp <= 0x200a) || // the typographic spaces
  (cp < 0x20 && cp !== 0x7f) ||
  cp === 0x7f

const nameOf = (cp: number) => NAMED[cp] ?? (cp === 0x09 ? 'tab' : cp === 0x0a ? 'line break' : cp === 0x0d ? 'carriage return' : cp >= 0x2000 && cp <= 0x200a ? 'typographic space' : cp < 0x20 || cp === 0x7f ? 'control character' : 'invisible character')

const codeOf = (cp: number) => 'U+' + cp.toString(16).toUpperCase().padStart(4, '0')

/** The hidden characters of text, in order; a space at the start or the end counts. */
export function findHidden(text: string): Hidden[] {
  const out: Hidden[] = []
  let i = 0
  for (const ch of text) {
    const cp = ch.codePointAt(0)!
    if (isInvisible(cp)) out.push({ index: i, code: codeOf(cp), name: nameOf(cp) })
    i += ch.length
  }
  const lead = text.length - text.trimStart().length
  const trail = text.length - text.trimEnd().length
  const blank = text.trim() === '' // only spaces: nothing to tell apart from "empty"
  if (!blank && text[0] === ' ') out.push({ index: 0, code: 'U+0020', name: `${lead} space${lead > 1 ? 's' : ''} at the start` })
  if (!blank && text[text.length - 1] === ' ') out.push({ index: text.length - trail, code: 'U+0020', name: `${trail} space${trail > 1 ? 's' : ''} at the end` })
  return out.sort((a, b) => a.index - b.index)
}

/** Text without its hidden characters and without a space at either end. */
export function stripHidden(text: string): string {
  let out = ''
  for (const ch of text) if (!isInvisible(ch.codePointAt(0)!)) out += ch
  return out.trim()
}

/** The text with each hidden character shown as ⟨U+200B⟩, as the import reports do. */
export function revealHidden(text: string): string {
  let out = ''
  for (const ch of text) out += isInvisible(ch.codePointAt(0)!) ? `⟨${codeOf(ch.codePointAt(0)!)}⟩` : ch
  return out
}
