import { describe, expect, it } from 'vitest'
import { NOTCH, wheelAction } from './wheel'

const ev = (o: Partial<Parameters<typeof wheelAction>[0]>) => ({ ctrlKey: false, shiftKey: false, deltaX: 0, deltaY: 0, deltaMode: 0, ...o })

describe('wheelAction', () => {
  it('zooms one step for a mouse notch, whichever way the browser counts it', () => {
    // Before: a Chromium notch (100) gave x0.37 and Firefox's line mode (3) about 3%.
    expect(wheelAction(ev({ ctrlKey: true, deltaY: -100 }))).toEqual({ kind: 'zoom', factor: NOTCH })
    expect(wheelAction(ev({ ctrlKey: true, deltaY: 100 }))).toEqual({ kind: 'zoom', factor: 1 / NOTCH })
    expect(wheelAction(ev({ ctrlKey: true, deltaY: -3, deltaMode: 1 }))).toEqual({ kind: 'zoom', factor: NOTCH })
  })
  it('zooms smoothly for a trackpad pinch (small, fractional deltas)', () => {
    const a = wheelAction(ev({ ctrlKey: true, deltaY: -2.5 }))
    expect(a.kind).toBe('zoom')
    if (a.kind === 'zoom') expect(a.factor).toBeCloseTo(Math.exp(0.025))
  })
  it('pans by the deltas for a two-finger scroll', () => {
    expect(wheelAction(ev({ deltaX: 4, deltaY: -7.5 }))).toEqual({ kind: 'pan', dx: 4, dy: -7.5 })
  })
  it('turns lines into pixels', () => {
    expect(wheelAction(ev({ deltaY: 3, deltaMode: 1 }))).toEqual({ kind: 'pan', dx: 0, dy: 48 })
  })
  it('pans sideways with Shift and a vertical wheel', () => {
    expect(wheelAction(ev({ shiftKey: true, deltaY: 100 }))).toEqual({ kind: 'pan', dx: 100, dy: 0 })
  })
})

// @ts-ignore
import canvas from '../Canvas.svelte?raw'

describe('canvas pointer', () => {
  // Before: a right click on the background started a pan and cleared the selection.
  it('ignores a right click on the background', () => {
    expect(canvas).toMatch(/function onbackground\(e: PointerEvent\) \{\s*if \(e\.button === 2\) return/)
  })
  it('reads wheel events through wheelAction', () => {
    expect(canvas).toContain('wheelAction(e)')
    expect(canvas).not.toContain('Math.exp(-e.deltaY')
  })
})
