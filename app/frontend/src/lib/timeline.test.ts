import { describe, expect, it } from 'vitest'
import { layoutBars, layoutGhosts, nextBoundary, spanOf, statusesAt, varsAt } from './timeline'
import type { GhostView, TimelineView } from './types'

const tl: TimelineView = {
  totalMS: 200,
  bars: [
    { id: 'login', type: 'request', status: 'passed', lane: 0, startMS: 0, endMS: 80, wrote: [] },
    { id: 'grab', type: 'set', status: 'passed', lane: 0, startMS: 80, endMS: 82, wrote: [{ name: 'token', value: 't-1' }] },
    { id: 'slow', type: 'request', status: 'failed', lane: 1, startMS: 90, endMS: 200, wrote: [] },
    { id: 'again', type: 'set', status: 'passed', lane: 0, startMS: 100, endMS: 101, wrote: [{ name: 'token', value: 't-2' }] },
  ],
}

describe('statusesAt', () => {
  it('shows nothing before the first step starts', () => {
    expect(statusesAt(tl.bars, -1)).toEqual({})
  })
  it('shows a step as running while the playhead is inside it and finished once past it', () => {
    expect(statusesAt(tl.bars, 40)).toEqual({ login: 'running' })
    expect(statusesAt(tl.bars, 95)).toEqual({ login: 'passed', grab: 'passed', slow: 'running' })
  })
  it('shows the final statuses at the end of the run', () => {
    expect(statusesAt(tl.bars, 200).slow).toBe('failed')
  })
})

describe('varsAt', () => {
  it('holds the value a variable had at that moment', () => {
    expect(varsAt(tl.bars, 50)).toEqual({})
    expect(varsAt(tl.bars, 95)).toEqual({ token: 't-1' })
    expect(varsAt(tl.bars, 150)).toEqual({ token: 't-2' })
  })
})

describe('nextBoundary', () => {
  it('steps to the next start or end, and back', () => {
    expect(nextBoundary(tl.bars, 0, 1)).toBe(80)
    expect(nextBoundary(tl.bars, 80, 1)).toBe(82)
    expect(nextBoundary(tl.bars, 82, -1)).toBe(80)
  })
  it('stays at the ends of the run', () => {
    expect(nextBoundary(tl.bars, 200, 1)).toBe(200)
    expect(nextBoundary(tl.bars, 0, -1)).toBe(0)
  })
})

describe('layoutBars', () => {
  it('scales time to width and lanes to rows, and keeps a short step visible', () => {
    const rects = layoutBars(tl, 400, 20)
    const grab = rects.find((r) => r.id === 'grab')!
    expect(grab.x).toBe(160)
    expect(grab.w).toBeGreaterThanOrEqual(3)
    expect(rects.find((r) => r.id === 'slow')!.y).toBe(20)
    expect(rects.find((r) => r.id === 'login')!.w).toBe(160)
  })
})

describe('layoutGhosts', () => {
  const ghost = (over: Partial<GhostView> = {}): GhostView => ({ id: 'login', lane: 1, status: 'passed', startMS: 100, endMS: 200, deltaMS: 0, ...over })

  it('puts a ghost on the same scale and lane as the bar it belongs behind', () => {
    const [r] = layoutGhosts([ghost()], 400, 200, 14)

    expect(r).toMatchObject({ x: 50, w: 50, y: 14 })
  })

  it('marks a step that took longer now as slower and a quicker one as faster', () => {
    const rects = layoutGhosts([ghost({ deltaMS: 30 }), ghost({ deltaMS: -30 }), ghost()], 400, 200, 14)

    expect(rects.map((r) => [r.slower, r.faster])).toEqual([[true, false], [false, true], [false, false]])
  })
})

describe('spanOf', () => {
  it('is long enough for an earlier run that was longer than this one', () => {
    const tl = { totalMS: 300, bars: [] }

    expect(spanOf(tl, [{ id: 'a', lane: 0, status: 'passed', startMS: 0, endMS: 900, deltaMS: 0 }])).toBe(900)
    expect(spanOf(tl, [])).toBe(300)
  })
})
