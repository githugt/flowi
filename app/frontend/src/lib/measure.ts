// Text that fits a width without a canvas: each character has a weight, in units of an average Latin letter.
// A CJK character or an emoji is two, a narrow letter (i, l, .) a little over half, a wide one (m, W) a third more.
// Before: the cards counted characters, so a line of CJK or capitals ran out of its card while a line of i's was
// clipped with room to spare.

const segmenter = typeof Intl !== 'undefined' && 'Segmenter' in Intl ? new Intl.Segmenter(undefined, { granularity: 'grapheme' }) : null

const NARROW = new Set("iljtfIr.,;:'!|()[]` ")
const WIDE = new Set('mwMW@%')
const WIDE_RANGES: [number, number][] = [
  [0x1100, 0x115f], [0x2e80, 0xa4cf], [0xac00, 0xd7a3], [0xf900, 0xfaff], [0xfe30, 0xfe6f], [0xff00, 0xff60], [0xffe0, 0xffe6], [0x20000, 0x3fffd],
]

/** The weight of one character (a grapheme: a letter with its accents, an emoji with its modifiers). */
export function charWidth(g: string): number {
  const cp = g.codePointAt(0) ?? 0
  if (/^\p{M}+$/u.test(g)) return 0
  if (/^\p{Regional_Indicator}/u.test(g) || /^\p{Emoji_Presentation}/u.test(g)) return 2
  if (WIDE_RANGES.some(([a, b]) => cp >= a && cp <= b)) return 2
  if (NARROW.has(g)) return 0.6
  if (WIDE.has(g)) return 1.3
  if (g >= 'A' && g <= 'Z') return 1.15
  return 1
}

function graphemes(text: string): string[] {
  return segmenter ? Array.from(segmenter.segment(text), (s) => s.segment) : Array.from(text)
}

function textWidth(text: string): number {
  return graphemes(text).reduce((n, g) => n + charWidth(g), 0)
}

/** `text` cut to `max` units of width, ending in an ellipsis when it was cut. Never splits a character. */
export function clipWidth(text: string, max: number): string {
  const chars = graphemes(text)
  if (chars.reduce((n, g) => n + charWidth(g), 0) <= max) return text
  let used = 0
  let out = ''
  for (const g of chars) {
    const w = charWidth(g)
    if (used + w > max - 1 && out) break
    out += g
    used += w
  }
  return out + '…'
}
