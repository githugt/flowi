import { describe, expect, it } from 'vitest'
import { axisTicks, formatSpan, slowestStep, timelineRows } from './timeline'

const bar = (step: string, startMs: number, endMs: number, status = 'passed') => ({ step, status, startMs, endMs })

describe('the run timeline', () => {
  it('should write a span in the unit that reads best', () => {
    // Before: a card said only "120ms"; there was no flow-level view to format.
    expect([formatSpan(0.2), formatSpan(120), formatSpan(1400), formatSpan(12_000), formatSpan(95_000)]).toEqual(['<1 ms', '120 ms', '1.4 s', '12 s', '1 min 35 s'])
  })

  it('should place each bar as a share of the run and flag the ones that overlap', () => {
    const rows = timelineRows([bar('a', 0, 100), bar('b', 50, 200), bar('c', 200, 400)])

    expect(rows.map((r) => [r.step, r.left, r.width, r.overlaps])).toEqual([
      ['a', 0, 25, false],
      ['b', 12.5, 37.5, true],
      ['c', 50, 50, false],
    ])
  })

  it('should keep a very short bar visible', () => {
    const rows = timelineRows([bar('slow', 0, 10_000), bar('quick', 5000, 5001)])

    expect(rows[1].width).toBe(0.5)
  })

  it('should draw round ticks that end inside the run', () => {
    const ticks = axisTicks(1000)

    expect(ticks.map((t) => t.label)).toEqual(['0', '200 ms', '400 ms', '600 ms', '800 ms', '1.0 s'])
    expect(ticks.at(-1)?.at).toBe(100)
    expect(axisTicks(0)).toEqual([])
  })

  it('should name the step that took most of the run, and nothing when the time is spread out', () => {
    expect(slowestStep([bar('fetch', 0, 900), bar('log', 900, 1000)])).toEqual({ step: 'fetch', share: 90 })
    expect(slowestStep([bar('a', 0, 100), bar('b', 100, 200), bar('c', 200, 300)])).toBeNull()
    expect(slowestStep([bar('only', 0, 100)])).toBeNull()
  })
})
