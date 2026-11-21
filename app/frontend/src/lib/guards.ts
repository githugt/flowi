import type { FaceGuard } from './types'

export interface PlacedGuard extends FaceGuard {
  x: number
  w: number
}

const CHAR = 6.6 // the width of one character of a chip, in canvas units
const PAD = 8 // padding inside a chip, both sides together
const GAP = 4

/**
 * Places the guard chips of a card at the right end of its sub-line, last guard against the edge, and says how much
 * room they take. A chip is as wide as its text; chips that would not fit in the card are dropped from the end, so
 * the sub-line never runs under them.
 */
export function placeGuards(guards: FaceGuard[] | undefined, cardWidth: number, left: number, right = 14): { chips: PlacedGuard[]; used: number } {
  const room = cardWidth - right - left - 60 // keep at least 60 units for the sub-line itself
  const sized = (guards ?? []).map((g) => ({ g, w: Math.ceil(g.glyph.length * CHAR + PAD) }))
  const kept: { g: FaceGuard; w: number }[] = []
  let total = 0
  for (const s of sized) {
    const next = total + s.w + (kept.length ? GAP : 0)
    if (next > room) break
    kept.push(s)
    total = next
  }
  let x = cardWidth - right - total
  const chips = kept.map((s) => {
    const placed = { ...s.g, x, w: s.w }
    x += s.w + GAP
    return placed
  })
  return { chips, used: total ? total + GAP : 0 }
}
