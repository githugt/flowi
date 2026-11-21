import { describe, expect, it } from 'vitest'
import wire from './WirePanel.svelte?raw'
import popover from './Popover.svelte?raw'

describe('WirePanel on Popover', () => {
  it('is a Popover in the data colour, with no layer, box, clamp or Escape handler of its own', () => {
    // Before: it wrote its own fixed box, z-index 30 and window-edge clamp, and closed on a window Escape handler,
    // with nothing behind it to take a press outside.
    expect(wire).toMatch(/<Popover [^>]*tone="data"/)
    expect(wire).not.toContain('use:modal')
    expect(wire).not.toContain('<svelte:window')
    expect(wire).not.toMatch(/z-index:\s*\d/)
    expect(popover).toContain("tone?: '' | 'data'")
    expect(popover).toContain('.popover.data { border-color: var(--data); }')
  })
})
