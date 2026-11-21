// How much of a card to draw at a zoom. At 30% a port is 2px and a title unreadable, so below TILE_BELOW a card is
// a tile with a big title and a status mark, and the ports are counter-scaled to stay a usable size.
// Before: every card drew all its rows, labels and ports at every zoom, so a zoomed-out flow was a grey smear.

export const TILE_BELOW = 0.45

export type Detail = 'full' | 'tile'

/** The level of detail at zoom `k` (1 is 100%). */
export function detailLevel(k: number): Detail {
  return k < TILE_BELOW ? 'tile' : 'full'
}

/** The radius of a port in canvas units at zoom `k`: 6 at normal sizes, larger when zoomed out so it stays about `minScreen`px wide, never over `max`. Whole numbers, so a zoom tick does not redraw every card. */
export function portRadius(k: number, base = 6, minScreen = 6, max = 14): number {
  if (!(k > 0)) return base
  return Math.round(Math.min(max, Math.max(base, minScreen / k)))
}
