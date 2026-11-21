import { describe, expect, it } from 'vitest'
import { modalOpen, nextTab, OPEN } from './modal'

describe('nextTab', () => {
  it('Given Tab at the last element Then focus wraps to the first, and Shift+Tab at the first to the last', () => {
    // Before: Tab walked out of a dialog into the page behind it.
    expect(nextTab(2, 3, false)).toBe(0)
    expect(nextTab(0, 3, true)).toBe(2)
  })
  it('Given Tab in the middle Then it moves one step either way', () => {
    expect(nextTab(0, 3, false)).toBe(1)
    expect(nextTab(2, 3, true)).toBe(1)
  })
  it('Given focus outside the dialog Then Tab enters at the first and Shift+Tab at the last', () => {
    expect(nextTab(-1, 3, false)).toBe(0)
    expect(nextTab(-1, 3, true)).toBe(2)
  })
  it('Given nothing to focus Then there is nowhere to go', () => {
    expect(nextTab(-1, 0, false)).toBeNull()
  })
})

describe('modalOpen', () => {
  it('Given a layer marked open Then a dialog is open, else not', () => {
    // Before: the canvas's Delete key still worked behind an open dialog.
    const open = { querySelector: (s: string) => (s === `[${OPEN}]` ? {} : null) } as unknown as ParentNode
    const none = { querySelector: () => null } as unknown as ParentNode

    expect(modalOpen(open)).toBe(true)
    expect(modalOpen(none)).toBe(false)
  })
})
