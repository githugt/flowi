import { describe, expect, it } from 'vitest'
import { MIN_VIEW_SHARE, minimapView, minimapWorthIt } from './minimap'

const extent = { x: 0, y: 0, w: 8000, h: 400 }

describe('minimapView', () => {
  it('keeps the visible area when it is big enough to see', () => {
    const shown = { x: 100, y: 0, w: 4000, h: 400 }
    expect(minimapView(shown, extent)).toEqual(shown)
  })
  it('grows a tiny rectangle to a share of the extent, around its centre', () => {
    // Before: the rectangle of a long flow was about 0.1 px wide.
    const v = minimapView({ x: 1000, y: 0, w: 100, h: 40 }, extent)
    expect(v.w).toBe(8000 * MIN_VIEW_SHARE)
    expect(v.x + v.w / 2).toBe(1050)
    expect(v.h).toBe(40) // 400 * 0.04 = 16, less than the 40 shown
  })
})

describe('minimapWorthIt', () => {
  it('is for a flow that is clipped, or one with more than 12 steps', () => {
    expect(minimapWorthIt(true, 3)).toBe(true)
    expect(minimapWorthIt(false, 13)).toBe(true)
    expect(minimapWorthIt(false, 5)).toBe(false)
    expect(minimapWorthIt(true, 1)).toBe(false)
  })
})

describe('the canvas', () => {
  it('draws the minimap rectangle with a stroke that does not shrink, and Esc clears the selection', async () => {
    // @ts-ignore node module in a vitest-only test
    const { readFileSync } = await import('node:fs')
    const canvas = readFileSync(new URL('../Canvas.svelte', import.meta.url), 'utf8') as string
    expect(canvas).toContain('vector-effect: non-scaling-stroke')
    expect(canvas).toContain('x={mini.x}')
    expect(canvas).toContain("onselect('', false)\n      return")
  })
})

describe('TextInput', () => {
  it('is as tall as a button (the control height token), so the two line up in one row', async () => {
    // Before: padding made a text field about 30 px and a button 28 px, and they sat misaligned in a row.
    // @ts-ignore node module in a vitest-only test
    const { readFileSync } = await import('node:fs')
    const field = readFileSync(new URL('../TextInput.svelte', import.meta.url), 'utf8') as string
    expect(field).toContain('height: var(--ctl-h)')
  })
})
