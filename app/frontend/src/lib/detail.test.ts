import { describe, expect, it } from 'vitest'
import { TILE_BELOW, detailLevel, portRadius } from './detail'
import canvas from '../Canvas.svelte?raw'
import card from '../StepCard.svelte?raw'

describe('canvas detail levels', () => {
  it('draws a tile below 45% and the whole card from there up', () => {
    // Before: every card drew every row and label at every zoom.
    expect(detailLevel(1)).toBe('full')
    expect(detailLevel(TILE_BELOW)).toBe('full')
    expect(detailLevel(0.3)).toBe('tile')
    expect(detailLevel(0.15)).toBe('tile')
  })

  it('grows the port as the canvas zooms out, to a limit, in whole numbers', () => {
    expect(portRadius(1)).toBe(6)
    expect(portRadius(0.5)).toBe(12)
    expect(portRadius(0.8)).toBe(8)
    expect(portRadius(0.15)).toBe(14)
    expect(portRadius(0)).toBe(6)
    expect(Number.isInteger(portRadius(0.37))).toBe(true)
  })

  it('is wired: the canvas passes the level and the card hides rows, sub-line and labels as a tile', () => {
    expect(canvas).toContain('detailLevel(view.k)')
    expect(canvas).toContain('detail={detail} portR={portR}')
    expect(card).toContain('tiled ? [] : face.rows')
    expect(card).toContain('class:big={tiled}')
  })
})
