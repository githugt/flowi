// The arrows between steps stacked in a loop's box. internal/canvas draws the same route (EdgePath, downPath) for
// hit-testing a drop, so the numbers here are the ones there.

/**
 * Canvas units: the gap between stacked steps, how far beside a step an arrow runs down, and the room in the box
 * around the steps (left and top, right for port labels, below for the "+ Add a step").
 */
export const STACK = { gap: 44, exitGutter: 10, gutterStep: 8, entryGutter: 12, inset: 24, portRoom: 56, addH: 44, backReach: 24 } as const

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
  const mid = top <= bottom ? bottom + STACK.gap / 4 : (bottom + top) / 2 // the next step is not below: still leave by the foot
  return viaMid(from, to, mid, port)
}

/** The six corners of an arrow that runs out of `from`, along the level `mid`, and into `to` from the left. */
function viaMid(from: Pt, to: Pt, mid: number, port: number, lane = 0): Pt[] {
  const out = from.x + STACK.exitGutter + Math.max(port, 0) * STACK.gutterStep
  const inn = to.x - STACK.entryGutter - Math.max(lane, 0) * STACK.gutterStep
  return [from, { x: out, y: from.y }, { x: out, y: mid }, { x: inn, y: mid }, { x: inn, y: to.y }, to]
}

/** Whether an arrow from a step's right edge `fromX` to a port at `toX` must turn back: the next step is not clear to its right. */
export const turnsBack = (fromX: number, toX: number): boolean => toX < fromX + STACK.backReach

/** A vertical extent: the top and the foot of a card. */
export interface Span {
  top: number
  bottom: number
}

/**
 * The corners of an arrow to a step that is behind or beside the one it leaves (a step dragged to the left of, above or
 * level with the step before it), drawn like a circuit trace with square corners instead of a curve that loops through
 * the cards: out of the port, up or down beside the card, across through the clear level between the two cards, and
 * into the port from the left. The level is the middle of the gap between them, whichever is above, and under both
 * when they overlap in height.
 */
export function elbowPoints(from: Pt, to: Pt, a: Span, b: Span, port = 0, lane = 0): Pt[] {
  if (b.top >= a.bottom) return viaMid(from, to, (a.bottom + b.top) / 2, port, lane)
  if (b.bottom <= a.top) return viaMid(from, to, (b.bottom + a.top) / 2, port, lane)
  return viaMid(from, to, Math.max(a.bottom, b.bottom) + STACK.gap / 4, port, lane)
}

/**
 * The corners of an arrow to a step clear to the right: along the port's level, up or down in the lane just before the
 * step, and on into its port. A step level with the port gets a straight line. The vertical is by the step, not in the
 * middle of the way, because the gap in front of a step is the one place known to be clear of cards (the middle of a
 * long way can be a card). `lane` moves it further from the step, so several arrows into one step run side by side;
 * a short way keeps it out of the exit lane of a later port.
 */
export function stepPoints(from: Pt, to: Pt, port = 0, lane = 0): Pt[] {
  if (Math.abs(from.y - to.y) < 1) return [from, to]
  const lo = from.x + STACK.exitGutter + Math.max(port, 0) * STACK.gutterStep
  const hi = to.x - STACK.entryGutter - Math.max(lane, 0) * STACK.gutterStep
  const x = lo <= hi ? hi : Math.max(Math.min(lo, to.x - STACK.entryGutter), (from.x + to.x) / 2)
  return [from, { x, y: from.y }, { x, y: to.y }, to]
}

/**
 * The square arrow between two steps, whichever way they lie: straight or stepped to a step on the right, round the
 * cards to one behind or beside. `lane` moves the last vertical of a way round further from the target, so several
 * arrows into one step run side by side and join only on the short stretch into its port.
 */
export function squarePoints(from: Pt, to: Pt, a: Span, b: Span, port = 0, lane = 0): Pt[] {
  return turnsBack(from.x, to.x) ? elbowPoints(from, to, a, b, port, lane) : stepPoints(from, to, port, lane)
}

/** The middle of an arrow's middle stretch, where its label and its + sit. */
export function routeCentre(route: Pt[]): Pt {
  if (route.length === 6) return { x: (route[2].x + route[3].x) / 2, y: route[2].y }
  if (route.length === 4) return { x: route[1].x, y: (route[1].y + route[2].y) / 2 }
  return { x: (route[0].x + route[route.length - 1].x) / 2, y: (route[0].y + route[route.length - 1].y) / 2 }
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
