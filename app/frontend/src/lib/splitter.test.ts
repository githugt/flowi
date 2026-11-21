import { describe, expect, it } from 'vitest'
import { SNAP, wantsCollapse, widthAfterKey } from './splitter'

describe('widthAfterKey', () => {
  it('widens a left pane with the Right arrow and narrows it with the Left', () => {
    expect(widthAfterKey(300, 'ArrowRight', false, 'left', 180, 720)).toBe(316)
    expect(widthAfterKey(300, 'ArrowLeft', false, 'left', 180, 720)).toBe(284)
  })
  it('works the other way round for a pane on the right', () => {
    expect(widthAfterKey(300, 'ArrowLeft', false, 'right', 180, 720)).toBe(316)
  })
  it('moves faster with Shift', () => {
    expect(widthAfterKey(300, 'ArrowRight', true, 'left', 180, 720)).toBe(364)
  })
  it('stays inside the limits', () => {
    expect(widthAfterKey(710, 'ArrowRight', true, 'left', 180, 720)).toBe(720)
    expect(widthAfterKey(190, 'ArrowLeft', true, 'left', 180, 720)).toBe(180)
  })
  it('goes to the limits with Home and End', () => {
    expect(widthAfterKey(300, 'Home', false, 'left', 180, 720)).toBe(180)
    expect(widthAfterKey(300, 'End', false, 'left', 180, 720)).toBe(720)
  })
  it('leaves other keys alone', () => {
    expect(widthAfterKey(300, 'a', false, 'left', 180, 720)).toBeNull()
  })
})

describe('widthAfterKey with a horizontal handle', () => {
  it('Given axis y When Up or Down is pressed Then the pane under the handle grows or shrinks', () => {
    // Before: the splitter only resized panes sideways, so the footer panel had a fixed height.
    expect(widthAfterKey(220, 'ArrowUp', false, 'left', 120, 600, 'y')).toBe(236)
    expect(widthAfterKey(220, 'ArrowDown', true, 'left', 120, 600, 'y')).toBe(156)
    expect(widthAfterKey(220, 'ArrowLeft', false, 'left', 120, 600, 'y')).toBeNull()
    expect(widthAfterKey(220, 'End', false, 'left', 120, 600, 'y')).toBe(600)
  })
})

describe('Splitter drag', () => {
  it('should end and keep the width when the system cancels the pointer, not only on release', async () => {
    // Before: a cancelled drag left its listeners on the handle and did not remember the width.
    const src = (await import('../Splitter.svelte?raw')).default as string

    for (const type of ['pointerup', 'pointercancel', 'lostpointercapture']) {
      expect(src).toContain(`el.addEventListener('${type}', up)`)
      expect(src).toContain(`el.removeEventListener('${type}', up)`)
    }
  })
})

describe('wantsCollapse', () => {
  // Before: a pane could not be folded by dragging; it stopped at its minimum.
  it('is false at and just below the minimum, so a drag to the limit does not fold', () => {
    expect(wantsCollapse(200, 200)).toBe(false)
    expect(wantsCollapse(200 - SNAP, 200)).toBe(false)
  })

  it('is true once the drag is more than the snap distance below the minimum', () => {
    expect(wantsCollapse(200 - SNAP - 1, 200)).toBe(true)
    expect(wantsCollapse(0, 200)).toBe(true)
  })

  it('takes another snap distance', () => {
    expect(wantsCollapse(150, 200, 40)).toBe(true)
    expect(wantsCollapse(170, 200, 40)).toBe(false)
  })
})
