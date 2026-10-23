import { describe, expect, it } from 'vitest'
import { markerAt, moved, slotAmong } from './reorder'

// Three other tabs: 0-100, 102-200 and 202-300 (middles 50, 151, 251).
const others = [
  { start: 0, end: 100 },
  { start: 102, end: 200 },
  { start: 202, end: 300 },
]

describe('slotAmong', () => {
  it.each([
    ['before every middle', -40, 0],
    ['past the first middle', 60, 1],
    ['between the second and third middles', 240, 2],
    ['past every middle', 900, 3],
  ])('Given the pointer %s Then it gives slot %i', (_, pos, want) => {
    const got = slotAmong(others, pos)

    expect(got).toBe(want)
  })

  it('Given no other items Then it gives slot 0', () => {
    expect(slotAmong([], 5)).toBe(0)
  })

  // A wide tab (A, 0-80) dragged past a narrow one (B): picking the tab under the
  // pointer swapped them, then swapped them back while the pointer stayed on B.
  it('Given a swap with a narrower neighbour When the pointer stays put Then the slot stays', () => {
    const pos = 110 // past the middle of B, at 80-130 before the swap
    const before = slotAmong([{ start: 80, end: 130 }], pos)

    const after = slotAmong([{ start: 0, end: 50 }], pos) // B after the swap

    expect([before, after]).toEqual([1, 1])
  })
})

describe('moved', () => {
  it('moves an item to a new place without changing the list given', () => {
    const list = ['a', 'b', 'c', 'd']

    expect(moved(list, 3, 1)).toEqual(['a', 'd', 'b', 'c'])
    expect(list).toEqual(['a', 'b', 'c', 'd'])
  })

  it('does nothing for the same place or an index outside the list', () => {
    expect(moved(['a', 'b'], 1, 1)).toEqual(['a', 'b'])
    expect(moved(['a', 'b'], 0, 5)).toEqual(['a', 'b'])
  })
})

describe('markerAt', () => {
  it.each([
    ['the first slot', 0, -1],
    ['a slot between two items', 2, 201],
    ['the slot after the last item', 3, 301],
  ])('Given %s Then the marker stands at its edge', (_, slot, want) => {
    const got = markerAt(others, slot, 0)

    expect(got).toBe(want)
  })

  it('Given no other items Then the marker stands at the fallback', () => {
    expect(markerAt([], 0, 42)).toBe(42)
  })
})
