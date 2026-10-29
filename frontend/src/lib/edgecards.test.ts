import { describe, expect, it } from 'vitest'
import { dropIndex, edgeCards, edgeHeight } from './edgecards'

const unit = (id: string, x: number, y: number, w = 280, h = 100) => ({ id, x, y, w, h })

describe('edgeHeight', () => {
  it('is the header, the sub-line and a row each, with room under the last', () => {
    expect(edgeHeight(0, false)).toBe(46 + 28 + 12)
    expect(edgeHeight(3, false)).toBe(46 + 28 + 3 * 34 + 12)
  })
  it('has one more row for the Add row while editing', () => {
    expect(edgeHeight(3, true)).toBe(edgeHeight(3, false) + 34)
  })
})

describe('edgeCards', () => {
  const units = [unit('b', 400, 80), unit('a', 40, 20), unit('c', 800, 60)]
  it('puts Start left of the leftmost card, level with its top', () => {
    const { start, first } = edgeCards(units, 2, 0, false)
    expect(first?.id).toBe('a')
    expect(start.x).toBe(40 - 80 - 280)
    expect(start.y).toBe(20)
  })
  it('puts End right of the rightmost card, level with its top', () => {
    const { end, last } = edgeCards(units, 2, 1, false)
    expect(last?.id).toBe('c')
    expect(end?.x).toBe(800 + 280 + 80)
    expect(end?.y).toBe(60)
  })
  it('draws no End card when there are no outputs and no ghost is wanted', () => {
    expect(edgeCards(units, 2, 0, false).end).toBeNull()
  })
  it('keeps room for the Add output ghost while editing', () => {
    const { end } = edgeCards(units, 2, 0, true)
    expect(end?.x).toBe(800 + 280 + 80)
    expect(end?.ghost).toBe(true)
  })
  it('puts Start at the origin when the flow has no cards', () => {
    const { start, first } = edgeCards([], 0, 0, true)
    expect(first).toBeUndefined()
    expect(start).toMatchObject({ x: 0, y: 0 })
  })
})

describe('dropIndex', () => {
  it('moves by whole rows of the zoomed height', () => {
    expect(dropIndex(0, 34 * 2, 1, 5)).toBe(2)
    expect(dropIndex(3, -34, 1, 5)).toBe(2)
  })
  it('allows for the zoom', () => {
    expect(dropIndex(0, 34, 0.5, 5)).toBe(2) // at half size a row is 17 pixels tall
  })
  it('stays inside the list', () => {
    expect(dropIndex(1, 9999, 1, 4)).toBe(3)
    expect(dropIndex(1, -9999, 1, 4)).toBe(0)
  })
  it('stays put for a small move', () => {
    expect(dropIndex(2, 10, 1, 5)).toBe(2)
  })
})
