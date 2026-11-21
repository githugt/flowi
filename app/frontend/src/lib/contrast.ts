/** WCAG contrast of colours, for the ink on a button whose fill the person can choose. */

const channel = (c: number) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4)

/** The relative luminance of a #rrggbb colour (0 for black, 1 for white). */
export function luminance(hex: string): number {
  const h = hex.replace('#', '')
  const [r, g, b] = [0, 2, 4].map((i) => channel(parseInt(h.slice(i, i + 2), 16) / 255))
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

/** The contrast ratio of two #rrggbb colours, from 1 to 21. */
export function contrastRatio(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}

const LIGHT_INK = '#ffffff'
const DARK_INK = '#10131a'

/** The ink, white or near-black, that reads better on a #rrggbb fill. */
export function readableInk(fill: string): string {
  return contrastRatio(fill, LIGHT_INK) >= contrastRatio(fill, DARK_INK) ? LIGHT_INK : DARK_INK
}

const HEX = /^#[0-9a-fA-F]{6}$/

/**
 * The ink to put on the accent when the person chose the accent themselves (in Settings or in a custom palette),
 * or '' when a theme's own accent and ink apply. The first colour that reads as #rrggbb wins.
 */
export function inkForChosenAccent(...accents: (string | undefined)[]): string {
  const own = accents.find((c) => c !== undefined && HEX.test(c))
  return own ? readableInk(own) : ''
}
