// The finished run in time (see bridge.TimelineView): what the strip under the canvas draws and what a playhead
// at a moment of the run means for the flow.
import type { BarView, GhostView, TimelineView } from './types'

/** What each step was doing at `t` milliseconds into the run: running inside it, its final status once past it. */
export function statusesAt(bars: BarView[], t: number): Record<string, string> {
  const out: Record<string, string> = {}
  for (const b of bars) {
    if (t >= b.endMS) out[b.id] = b.status
    else if (t >= b.startMS) out[b.id] = 'running'
  }
  return out
}

/** The variables as they stood at `t`: the writes of every step that ended by then, oldest first. */
export function varsAt(bars: BarView[], t: number): Record<string, string> {
  const out: Record<string, string> = {}
  const done = bars.filter((b) => b.endMS <= t && b.wrote.length).sort((a, b) => a.endMS - b.endMS)
  for (const b of done) for (const w of b.wrote) out[w.name] = w.value
  return out
}

/** The next moment a step starts or ends after `t` (before it when dir is -1); `t` itself at the ends of the run. */
export function nextBoundary(bars: BarView[], t: number, dir: 1 | -1): number {
  const marks = [...new Set(bars.flatMap((b) => [b.startMS, b.endMS]))].sort((a, b) => a - b)
  const found = dir === 1 ? marks.find((m) => m > t) : [...marks].reverse().find((m) => m < t)
  return found ?? t
}

export interface BarRect {
  id: string
  status: string
  x: number
  y: number
  w: number
  h: number
}

const MIN_W = 3

/** The bars as rectangles in a strip `width` wide with `laneH` high lanes. */
export function layoutBars(tl: TimelineView, width: number, laneH: number, span = tl.totalMS): BarRect[] {
  const total = Math.max(span, 1)
  return tl.bars.map((b) => ({
    id: b.id,
    status: b.status,
    x: (b.startMS / total) * width,
    y: b.lane * laneH,
    w: Math.max(((b.endMS - b.startMS) / total) * width, MIN_W),
    h: laneH,
  }))
}

/** The earlier run's bars as rectangles on the same scale as the current run's, so the two line up. A run that was
 *  longer than this one sets the scale: `total` is the larger of the two lengths, and the current bars use it too. */
export function layoutGhosts(ghosts: GhostView[], total: number, width: number, laneH: number): (BarRect & { slower: boolean; faster: boolean })[] {
  const span = Math.max(total, 1)
  return ghosts.map((g) => ({
    id: g.id,
    status: g.status,
    x: (g.startMS / span) * width,
    y: g.lane * laneH,
    w: Math.max(((g.endMS - g.startMS) / span) * width, MIN_W),
    h: laneH,
    slower: g.deltaMS > 0,
    faster: g.deltaMS < 0,
  }))
}

/** How long the strip must be to hold both runs: the later end of the current bars and of the ghosts. */
export function spanOf(tl: TimelineView, ghosts: GhostView[]): number {
  return Math.max(tl.totalMS, ...ghosts.map((g) => g.endMS), 1)
}
