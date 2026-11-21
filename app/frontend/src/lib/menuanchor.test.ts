import { describe, expect, it } from 'vitest'
import { anchorAt, menuPlace } from './menuanchor'

const button = { left: 100, right: 160, top: 40, bottom: 64 }

describe('anchorAt', () => {
  it('opens under the button by its left edge unless told otherwise', () => {
    expect(anchorAt(button)).toEqual({ x: 100, y: 68, end: undefined, above: undefined })
  })

  it('can put the right edge of the menu on the right edge of the button', () => {
    expect(anchorAt(button, { end: true })).toMatchObject({ x: 160, end: true })
  })

  it('can open upward, with the menu ending just over the button', () => {
    expect(anchorAt(button, { above: true })).toMatchObject({ y: 36, above: true })
  })
})

describe('menuPlace', () => {
  const room = { w: 800, h: 600 }

  it('opens over the button when it does not fit below and there is more room above', () => {
    const low = anchorAt({ left: 100, right: 160, top: 500, bottom: 524 }, { flip: true })

    expect(menuPlace(low, { w: 200, h: 120 }, room)).toEqual({ left: 100, top: 376 })
  })

  it('stays under the button when it fits, even if told it may flip', () => {
    expect(menuPlace(anchorAt(button, { flip: true }), { w: 200, h: 120 }, room)).toEqual({ left: 100, top: 68 })
  })

  it('stays under the button when there is no more room above', () => {
    const tall = anchorAt({ left: 100, right: 160, top: 100, bottom: 124 }, { flip: true })

    expect(menuPlace(tall, { w: 200, h: 560 }, room)).toEqual({ left: 100, top: 32 })
  })

  it('starts at the point when the menu fits', () => {
    expect(menuPlace({ x: 100, y: 68 }, { w: 200, h: 120 }, room)).toEqual({ left: 100, top: 68 })
  })

  it('ends at the point for end and above', () => {
    expect(menuPlace({ x: 460, y: 300, end: true, above: true }, { w: 200, h: 120 }, room)).toEqual({ left: 260, top: 180 })
  })

  it('is pulled back inside when it would run off the right or the bottom', () => {
    expect(menuPlace({ x: 790, y: 590 }, { w: 200, h: 120 }, room)).toEqual({ left: 592, top: 472 })
  })

  it('starts at the margin when it is bigger than the window', () => {
    expect(menuPlace({ x: 50, y: 50 }, { w: 900, h: 700 }, room)).toEqual({ left: 8, top: 8 })
  })
})
