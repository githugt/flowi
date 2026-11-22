// The arrows between steps stacked in a loop's box. internal/canvas draws the same route (EdgePath, downPath) for
// hit-testing a drop, so the numbers here are the ones there.

/**
 * Canvas units: the gap between stacked steps, how far beside a step an arrow runs down, and the room in the box
 * around the steps (left and top, right for port labels, below for the "+ Add a step").
 */
export const STACK = { gap: 44, exitGutter: 10, gutterStep: 8, entryGutter: 12, inset: 24, portRoom: 56, addH: 44 } as const

export interface Pt {
  x: number
  y: number
}

/**
 * The corners of the way from a step's out port `from` to the in port `to` of the one stacked under it: right of the
 * card, down, left through the gap between the two cards, down beside the next card and into its in port. `bottom` is
 * the foot of the first card and `top` the head of the second. A port further down the card (`port`, 0 for the first)
 * runs further out, so two arrows of one step do not overlap.
 */
export function downPoints(from: Pt, to: Pt, bottom: number, top: number, port = 0): Pt[] {
  const out = from.x + STACK.exitGutter + Math.max(port, 0) * STACK.gutterStep
  const inn = to.x - STACK.entryGutter
  const mid = top <= bottom ? bottom + STACK.gap / 4 : (bottom + top) / 2 // the next step is not below: still leave by the foot
  return [from, { x: out, y: from.y }, { x: out, y: mid }, { x: inn, y: mid }, { x: inn, y: to.y }, to]
}

/** An SVG path through the points, its corners rounded by up to `radius` (less where a side is short). */
export function roundedPath(points: Pt[], radius = 8): string {
  if (points.length < 2) return ''
  let d = `M${points[0].x},${points[0].y}`
  for (let i = 1; i < points.length - 1; i++) {
    const prev = points[i - 1]
    const cur = points[i]
    const next = points[i + 1]
    const before = Math.hypot(cur.x - prev.x, cur.y - prev.y)
    const after = Math.hypot(next.x - cur.x, next.y - cur.y)
    const r = Math.min(radius, before / 2, after / 2)
    if (r < 1) {
      d += ` L${cur.x},${cur.y}`
      continue
    }
    const a = { x: cur.x + ((prev.x - cur.x) / before) * r, y: cur.y + ((prev.y - cur.y) / before) * r }
    const b = { x: cur.x + ((next.x - cur.x) / after) * r, y: cur.y + ((next.y - cur.y) / after) * r }
    d += ` L${a.x},${a.y} Q${cur.x},${cur.y} ${b.x},${b.y}`
  }
  const last = points[points.length - 1]
  return `${d} L${last.x},${last.y}`
}
