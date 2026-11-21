import { describe, expect, it } from 'vitest'
import { ZOOM_MAX, ZOOM_MIN, zoomForKey } from './zoom'

const ctrl = (key: string) => ({ key, ctrlKey: true, metaKey: false })

describe('zoomForKey', () => {
  it('steps up with + or =, down with -, and resets with 0', () => {
    expect(zoomForKey(ctrl('+'), 1)).toBe(1.1)
    expect(zoomForKey(ctrl('='), 1)).toBe(1.1)
    expect(zoomForKey(ctrl('-'), 1)).toBe(0.9)
    expect(zoomForKey(ctrl('0'), 1.4)).toBe(1)
  })
  it('stays between the limits', () => {
    expect(zoomForKey(ctrl('+'), ZOOM_MAX)).toBe(ZOOM_MAX)
    expect(zoomForKey(ctrl('-'), ZOOM_MIN)).toBe(ZOOM_MIN)
  })
  it('ignores a key without Ctrl or Cmd, and other keys', () => {
    expect(zoomForKey({ key: '+', ctrlKey: false, metaKey: false }, 1)).toBeNull()
    expect(zoomForKey(ctrl('a'), 1)).toBeNull()
  })
  it('accepts Cmd on a Mac', () => {
    expect(zoomForKey({ key: '-', ctrlKey: false, metaKey: true }, 1)).toBe(0.9)
  })
})
