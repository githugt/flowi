import { describe, expect, it } from 'vitest'
import { STACK, downPoints, roundedPath } from './stackpath'

describe('downPoints', () => {
  it('runs out of the port, down beside the card, through the gap and into the next card from the left', () => {
    const pts = downPoints({ x: 280, y: 23 }, { x: 0, y: 150 }, 100, 140)

    expect(pts[0]).toEqual({ x: 280, y: 23 })
    expect(pts[1]).toEqual({ x: 280 + STACK.exitGutter, y: 23 })
    expect(pts[2]).toEqual({ x: 280 + STACK.exitGutter, y: 120 }) // the middle of the gap between 100 and 140
    expect(pts[3]).toEqual({ x: -STACK.entryGutter, y: 120 })
    expect(pts[4]).toEqual({ x: -STACK.entryGutter, y: 150 })
    expect(pts[5]).toEqual({ x: 0, y: 150 })
  })

  it('runs a later port further out, so the arrows of one step do not overlap', () => {
    const first = downPoints({ x: 280, y: 23 }, { x: 0, y: 150 }, 100, 140, 0)
    const second = downPoints({ x: 280, y: 47 }, { x: 0, y: 150 }, 100, 140, 1)

    expect(second[1].x - first[1].x).toBe(STACK.gutterStep)
  })

  it('leaves by the foot of the card when the next step is not below it', () => {
    const pts = downPoints({ x: 280, y: 23 }, { x: 0, y: 20 }, 100, 10)

    expect(pts[2].y).toBe(100 + STACK.gap / 4)
  })
})

describe('roundedPath', () => {
  it('draws a straight line through two points', () => {
    expect(roundedPath([{ x: 0, y: 0 }, { x: 10, y: 0 }])).toBe('M0,0 L10,0')
  })

  it('rounds a corner with a curve that starts and ends on its sides', () => {
    const d = roundedPath([{ x: 0, y: 0 }, { x: 20, y: 0 }, { x: 20, y: 20 }], 4)

    expect(d).toBe('M0,0 L16,0 Q20,0 20,4 L20,20')
  })

  it('keeps a corner sharp where a side is too short to round', () => {
    const d = roundedPath([{ x: 0, y: 0 }, { x: 1, y: 0 }, { x: 1, y: 20 }], 8)

    expect(d).toBe('M0,0 L1,0 L1,20')
  })

  it('gives no path for fewer than two points', () => {
    expect(roundedPath([{ x: 0, y: 0 }])).toBe('')
  })
})
