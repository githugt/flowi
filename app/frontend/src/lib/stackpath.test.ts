import { describe, expect, it } from 'vitest'
import { STACK, downPoints, elbowPoints, roundedPath, routeCentre, squarePoints, stepPoints, turnsBack } from './stackpath'

// Before: an arrow to a step dragged behind or under the step it follows was a curve that swung out of the port, looped
// back and ran diagonally behind the cards. It is now routed with square corners, like the stacked steps in a loop.
describe('turnsBack', () => {
  it('is false for a step with room to the right of the one it follows', () => {
    expect(turnsBack(280, 360)).toBe(false)
  })

  it('is true for a step to the left, level with, or too close to the right edge', () => {
    expect(turnsBack(280, 40)).toBe(true)
    expect(turnsBack(280, 280)).toBe(true)
    expect(turnsBack(280, 280 + STACK.backReach - 1)).toBe(true)
  })
})

describe('elbowPoints', () => {
  const from = { x: 280, y: 23 }

  it('runs through the gap between the cards to a step below and behind, entering from the left', () => {
    const pts = elbowPoints(from, { x: 40, y: 253 }, { top: 0, bottom: 100 }, { top: 230, bottom: 330 })

    expect(pts[2]).toEqual({ x: 280 + STACK.exitGutter, y: 165 }) // the middle of the gap between 100 and 230
    expect(pts[3]).toEqual({ x: 40 - STACK.entryGutter, y: 165 })
    expect(pts[4]).toEqual({ x: 40 - STACK.entryGutter, y: 253 })
    expect(pts[5]).toEqual({ x: 40, y: 253 })
  })

  it('goes up through the gap to a step above and behind, without passing under the card it leaves', () => {
    const pts = elbowPoints({ x: 280, y: 323 }, { x: 40, y: 23 }, { top: 300, bottom: 400 }, { top: 0, bottom: 100 })

    expect(pts[2].y).toBe(200) // between the foot of the step above (100) and the head of this one (300)
  })

  it('runs under both when the steps overlap in height, one behind the other on a row', () => {
    const pts = elbowPoints(from, { x: 40, y: 23 }, { top: 0, bottom: 100 }, { top: 10, bottom: 130 })

    expect(pts[2].y).toBe(130 + STACK.gap / 4)
  })

  it('keeps downPoints as it was for a step stacked below', () => {
    const a = elbowPoints(from, { x: 0, y: 150 }, { top: 0, bottom: 100 }, { top: 140, bottom: 200 })
    const b = downPoints(from, { x: 0, y: 150 }, 100, 140)

    expect(a).toEqual(b)
  })

  it('runs a later port further out, as downPoints does', () => {
    const first = elbowPoints({ x: 280, y: 23 }, { x: 40, y: 253 }, { top: 0, bottom: 100 }, { top: 230, bottom: 330 }, 0)
    const second = elbowPoints({ x: 280, y: 47 }, { x: 40, y: 253 }, { top: 0, bottom: 100 }, { top: 230, bottom: 330 }, 1)

    expect(second[1].x - first[1].x).toBe(STACK.gutterStep)
  })
})

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

describe('lanes into one step', () => {
  const a = { top: 0, bottom: 100 }
  const b = { top: 230, bottom: 330 }

  it('moves the last vertical of a way round further from the step for a later lane, so arrows into it do not share a line', () => {
    const first = elbowPoints({ x: 280, y: 23 }, { x: 40, y: 253 }, a, b, 0, 0)
    const second = elbowPoints({ x: 280, y: 23 }, { x: 40, y: 253 }, a, b, 0, 1)

    expect(first[3].x - second[3].x).toBe(STACK.gutterStep)
    expect(second[4].x).toBe(second[3].x) // still one vertical, into the port by a short stretch
  })
})

describe('stepPoints', () => {
  it('is a straight line to a step level with the port', () => {
    expect(stepPoints({ x: 280, y: 23 }, { x: 360, y: 23 })).toEqual([{ x: 280, y: 23 }, { x: 360, y: 23 }])
  })

  it('goes along the port, steps up or down in the lane just before the next step, and on into its port', () => {
    const pts = stepPoints({ x: 280, y: 23 }, { x: 400, y: 123 })

    expect(pts).toEqual([{ x: 280, y: 23 }, { x: 388, y: 23 }, { x: 388, y: 123 }, { x: 400, y: 123 }])
  })

  it('moves the vertical further from the step for a later lane, so arrows into one step run side by side', () => {
    const first = stepPoints({ x: 280, y: 23 }, { x: 400, y: 123 }, 0, 0)
    const second = stepPoints({ x: 280, y: 23 }, { x: 400, y: 223 }, 0, 1)

    expect(first[1].x - second[1].x).toBe(STACK.gutterStep)
  })

  it('keeps the vertical out of the exit lane of a later port when the way is short', () => {
    const pts = stepPoints({ x: 280, y: 23 }, { x: 320, y: 123 }, 2, 2)

    expect(pts[1].x).toBe(280 + STACK.exitGutter + 2 * STACK.gutterStep)
  })

  it('falls back to the middle of the way when there is no room for both lanes', () => {
    expect(stepPoints({ x: 280, y: 23 }, { x: 306, y: 123 }, 2)[1].x).toBe(294)
  })
})

describe('squarePoints', () => {
  const a = { top: 0, bottom: 100 }

  it('steps to a step clear to the right and goes round the cards to one behind', () => {
    const forward = squarePoints({ x: 280, y: 23 }, { x: 400, y: 123 }, a, { top: 100, bottom: 200 })
    const back = squarePoints({ x: 280, y: 23 }, { x: 40, y: 253 }, a, { top: 230, bottom: 330 })

    expect(forward).toHaveLength(4)
    expect(back).toHaveLength(6)
  })
})

describe('routeCentre', () => {
  it('is the middle of the middle stretch, for a way round, a step and a straight line', () => {
    expect(routeCentre(elbowPoints({ x: 280, y: 23 }, { x: 40, y: 253 }, { top: 0, bottom: 100 }, { top: 230, bottom: 330 }))).toEqual({ x: (290 + 28) / 2, y: 165 })
    expect(routeCentre(stepPoints({ x: 280, y: 23 }, { x: 400, y: 123 }))).toEqual({ x: 388, y: 73 })
    expect(routeCentre([{ x: 0, y: 0 }, { x: 10, y: 10 }])).toEqual({ x: 5, y: 5 })
  })
})
