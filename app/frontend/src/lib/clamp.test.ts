import { describe, expect, it } from 'vitest'
import { clamp, keepInside } from './clamp'

describe('clamp', () => {
  it('holds a number between the bounds', () => {
    expect(clamp(5, 0, 10)).toBe(5)
    expect(clamp(-3, 0, 10)).toBe(0)
    expect(clamp(99, 0, 10)).toBe(10)
  })
  it('gives the low bound when the high one is below it, as for an empty list', () => {
    expect(clamp(0, 0, -1)).toBe(0)
  })
})

describe('keepInside', () => {
  it('keeps a popup inside the window, a margin from the edges', () => {
    expect(keepInside(300, 200, 1000)).toBe(300)
    expect(keepInside(950, 200, 1000)).toBe(792)
    expect(keepInside(-20, 200, 1000)).toBe(8)
  })
  it('starts a popup bigger than the room at the margin', () => {
    expect(keepInside(100, 2000, 1000)).toBe(8)
  })
})
