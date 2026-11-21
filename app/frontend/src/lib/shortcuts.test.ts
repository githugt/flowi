import { describe, expect, it } from 'vitest'
import { shortcutSheet, windowKeyAction } from './shortcuts'

const k = (key: string, o: Partial<Parameters<typeof windowKeyAction>[0]> = {}) => ({ key, ctrlKey: false, metaKey: false, altKey: false, shiftKey: false, ...o })

describe('windowKeyAction', () => {
  // Before: Ctrl+Tab and Ctrl+1..9 did nothing, and there was no list of shortcuts.
  it('cycles tabs with Ctrl+Tab and Ctrl+Shift+Tab', () => {
    expect(windowKeyAction(k('Tab', { ctrlKey: true }), false)).toBe('next-tab')
    expect(windowKeyAction(k('Tab', { ctrlKey: true, shiftKey: true }), false)).toBe('previous-tab')
  })
  it('goes to a tab by number with Ctrl or Cmd', () => {
    expect(windowKeyAction(k('3', { ctrlKey: true }), false)).toBe('tab-3')
    expect(windowKeyAction(k('9', { metaKey: true }), true)).toBe('tab-9')
    expect(windowKeyAction(k('0', { ctrlKey: true }), false)).toBe('')
  })
  it('opens the sheet on ? but never while typing', () => {
    expect(windowKeyAction(k('?', { shiftKey: true }), false)).toBe('shortcuts')
    expect(windowKeyAction(k('?', { shiftKey: true }), true)).toBe('')
  })
  it('leaves other keys alone', () => {
    expect(windowKeyAction(k('a'), false)).toBe('')
    expect(windowKeyAction(k('Tab'), false)).toBe('')
  })
})

describe('shortcutSheet', () => {
  it('lists the menu shortcuts and the canvas gestures', () => {
    const text = shortcutSheet(false)
    expect(text).toContain('Ctrl+K   Jump to…')
    expect(text).toContain('Ctrl+Enter   Send the request')
    expect(text).toContain('Drag from a port to a step')
  })
  it('writes Cmd keys on a Mac', () => {
    expect(shortcutSheet(true)).toContain('⌘K   Jump to…')
    expect(shortcutSheet(true)).toContain('⇧⌘N   New flow')
  })
})
