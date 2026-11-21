import { describe, expect, it } from 'vitest'
import { relayoutLabel, relayoutNote } from './relayout'

describe('relayoutLabel', () => {
  it('says all when nothing is selected, and counts the selection otherwise', () => {
    // Before: the label was "Re-layout" in both cases.
    expect(relayoutLabel(0)).toBe('Re-layout all steps')
    expect(relayoutLabel(1)).toBe('Re-layout 1 selected step')
    expect(relayoutLabel(3)).toBe('Re-layout 3 selected steps')
  })
})

describe('relayoutNote', () => {
  it('names how many steps moved and how to undo', () => {
    expect(relayoutNote(0, 12)).toBe('Re-laid out 12 steps. Undo brings the old places back.')
    expect(relayoutNote(1, 12)).toBe('Re-laid out 1 step. Undo brings the old places back.')
  })
})
