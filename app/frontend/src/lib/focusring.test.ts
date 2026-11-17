import { describe, expect, it } from 'vitest'
import { dropPointerFocusOnBlur } from './focusring'

// A window with only what the function touches: listeners, and a focused element that can be blurred.
function fakeWindow(focused: { text: boolean }) {
  const listeners: Record<string, (e: unknown) => void> = {}
  let blurred = false
  const el = { matches: () => focused.text, blur: () => (blurred = true) }
  const win = { addEventListener: (type: string, f: (e: unknown) => void) => (listeners[type] = f), document: { activeElement: el } }
  dropPointerFocusOnBlur(win as unknown as Window)
  return { fire: (type: string, e: unknown = {}) => listeners[type](e), blurred: () => blurred }
}

describe('dropPointerFocusOnBlur', () => {
  // Before: a clicked tab kept focus while the window was away and drew a focus ring when it came back.
  it('lets go of a clicked control when the window loses focus, Alt on the way to Alt+Tab included', () => {
    const w = fakeWindow({ text: false })

    w.fire('pointerdown')
    w.fire('keydown', { key: 'Alt' })
    w.fire('blur')

    expect(w.blurred()).toBe(true)
  })

  it('keeps focus on a control reached by keyboard', () => {
    const w = fakeWindow({ text: false })

    w.fire('pointerdown')
    w.fire('keydown', { key: 'Tab' })
    w.fire('blur')

    expect(w.blurred()).toBe(false)
  })

  it('keeps focus in a text field, so the caret stays where it was', () => {
    const w = fakeWindow({ text: true })

    w.fire('pointerdown')
    w.fire('blur')

    expect(w.blurred()).toBe(false)
  })
})
