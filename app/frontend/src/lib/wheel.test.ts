import { describe, expect, it } from 'vitest'
import { NOTCH, wheelReader } from './wheel'

const ev = (o: Partial<Parameters<ReturnType<typeof wheelReader>>[0]>) => ({ ctrlKey: false, shiftKey: false, deltaX: 0, deltaY: 0, deltaMode: 0, timeStamp: 1000, ...o })
const at = (t: number, o: Parameters<typeof ev>[0]) => ev({ timeStamp: t, ...o })

describe('wheelReader', () => {
  it('zooms one step for a plain mouse notch, whichever amount the system sends', () => {
    // Before: a plain notch panned, and Ctrl+notch gave x0.37 in Chromium and about 3% in Firefox's line mode.
    for (const dy of [100, 120, 53, 125]) {
      const read = wheelReader()
      expect(read(ev({ deltaY: -dy }))).toEqual({ kind: 'zoom', factor: NOTCH })
      expect(read(at(5000, { deltaY: dy }))).toEqual({ kind: 'zoom', factor: 1 / NOTCH })
    }
    expect(wheelReader()(ev({ deltaY: -3, deltaMode: 1 }))).toEqual({ kind: 'zoom', factor: NOTCH })
  })
  it('keeps zooming while the wheel is spun, as each notch sends the same amount', () => {
    const read = wheelReader()
    read(at(1000, { deltaY: 53 }))

    expect(read(at(1030, { deltaY: 53 }))).toEqual({ kind: 'zoom', factor: 1 / NOTCH })
  })
  it('pans with Ctrl and a mouse notch', () => {
    expect(wheelReader()(ev({ ctrlKey: true, deltaY: 100 }))).toEqual({ kind: 'pan', dx: 0, dy: 100 })
    expect(wheelReader()(ev({ ctrlKey: true, deltaY: 3, deltaMode: 1 }))).toEqual({ kind: 'pan', dx: 0, dy: 48 })
  })
  it('zooms smoothly for a trackpad pinch (Ctrl with small, fractional deltas)', () => {
    const a = wheelReader()(ev({ ctrlKey: true, deltaY: -2.5 }))
    expect(a.kind).toBe('zoom')
    if (a.kind === 'zoom') expect(a.factor).toBeCloseTo(Math.exp(0.025))
  })
  it('pans by the deltas for a two-finger scroll', () => {
    expect(wheelReader()(ev({ deltaX: 4, deltaY: -7.5 }))).toEqual({ kind: 'pan', dx: 4, dy: -7.5 })
  })
  it('does not zoom for a fast trackpad flick, whose whole numbers vary from event to event', () => {
    const read = wheelReader()
    read(at(1000, { deltaY: 12 }))

    expect(read(at(1016, { deltaY: 87 })).kind).toBe('pan')
    expect(read(at(1032, { deltaY: 140 })).kind).toBe('pan')
    expect(wheelReader()(ev({ deltaX: 3, deltaY: 200 })).kind).toBe('pan')
  })
  it('pans sideways with Shift and a vertical wheel', () => {
    expect(wheelReader()(ev({ shiftKey: true, deltaY: 100 }))).toEqual({ kind: 'pan', dx: 100, dy: 0 })
  })
})

// @ts-ignore
import canvas from '../Canvas.svelte?raw'

describe('canvas pointer', () => {
  // Before: a right click on the background started a pan and cleared the selection.
  it('ignores a right click on the background', () => {
    expect(canvas).toMatch(/function onbackground\(e: PointerEvent\) \{\s*if \(e\.button === 2\) return/)
  })
  it('reads wheel events through wheelReader', () => {
    expect(canvas).toContain('readWheel(e)')
    expect(canvas).not.toContain('Math.exp(-e.deltaY')
  })
})
