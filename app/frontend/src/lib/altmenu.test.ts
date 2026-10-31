import { describe, expect, it, vi } from 'vitest'
import { createAltTap, isWindows } from './altmenu'

const press = (key: string, repeat = false) => ({ key, repeat })

describe('createAltTap', () => {
  it('fires once when Alt goes down and up alone', () => {
    const ontap = vi.fn()
    const tap = createAltTap(ontap)

    tap.keydown(press('Alt'))
    tap.keyup(press('Alt'))

    expect(ontap).toHaveBeenCalledTimes(1)
  })

  it('does not fire when another key was pressed with Alt', () => {
    const ontap = vi.fn()
    const tap = createAltTap(ontap)

    tap.keydown(press('Alt'))
    tap.keydown(press('Tab'))
    tap.keyup(press('Tab'))
    tap.keyup(press('Alt'))

    expect(ontap).not.toHaveBeenCalled()
  })

  it('does not fire when a click came between, or the window lost focus', () => {
    const ontap = vi.fn()
    const tap = createAltTap(ontap)

    tap.keydown(press('Alt'))
    tap.cancel()
    tap.keyup(press('Alt'))

    expect(ontap).not.toHaveBeenCalled()
  })

  it('still fires after Alt is held long enough to repeat', () => {
    const ontap = vi.fn()
    const tap = createAltTap(ontap)

    tap.keydown(press('Alt'))
    tap.keydown(press('Alt', true))
    tap.keyup(press('Alt'))

    expect(ontap).toHaveBeenCalledTimes(1)
  })

  it('starts fresh after a tap', () => {
    const ontap = vi.fn()
    const tap = createAltTap(ontap)

    tap.keydown(press('Alt'))
    tap.keyup(press('Alt'))
    tap.keyup(press('Alt'))

    expect(ontap).toHaveBeenCalledTimes(1)
  })
})

describe('isWindows', () => {
  it('recognises the Windows user agent only', () => {
    expect(isWindows('Mozilla/5.0 (Windows NT 10.0; Win64; x64) Edg/120')).toBe(true)
    expect(isWindows('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)')).toBe(false)
    expect(isWindows('Mozilla/5.0 (X11; Linux x86_64)')).toBe(false)
  })
})
