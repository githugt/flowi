// The rectangle that marks the part of the flow on screen in the minimap.
// Before: on a flow 8000 units wide the rectangle was about a tenth of a pixel across, with a stroke scaled down
// with it, so it could not be seen or grabbed.

export interface Box {
  x: number
  y: number
  w: number
  h: number
}

/** The share of the flow's width and height a view rectangle is never drawn smaller than. */
export const MIN_VIEW_SHARE = 0.04

/** The visible area as drawn in the minimap: at least MIN_VIEW_SHARE of the extent each way, around the same centre. */
export function minimapView(shown: Box, extent: Box): Box {
  const w = Math.max(shown.w, extent.w * MIN_VIEW_SHARE)
  const h = Math.max(shown.h, extent.h * MIN_VIEW_SHARE)
  return { x: shown.x + shown.w / 2 - w / 2, y: shown.y + shown.h / 2 - h / 2, w, h }
}

/** Whether the minimap is worth drawing: when part of the flow is out of view, or there are more steps than a glance holds. */
export function minimapWorthIt(clipped: boolean, units: number): boolean {
  return units > 1 && (clipped || units > 12)
}
