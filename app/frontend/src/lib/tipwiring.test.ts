import { describe, expect, it } from 'vitest'
import tip from '../Tip.svelte?raw'
import hint from '../Hint.svelte?raw'
import kvrows from '../KVRows.svelte?raw'

describe('Tip', () => {
  it('shows on pointer rest or keyboard focus, goes on Escape, and is described to a screen reader as a tooltip', () => {
    expect(tip).toContain('onmouseenter={show}')
    expect(tip).toContain('onfocusin={show}')
    expect(tip).toContain('onkeydowncapture={escape}')
    expect(tip).toContain("e.key !== 'Escape'")
    expect(tip).toContain('role="tooltip"')
    expect(tip).toContain('aria-describedby')
  })

  it('sits inside the window and above dialogs, from the z-index token', () => {
    expect(tip).toContain('window.innerWidth - width')
    expect(tip).toContain('var(--z-top)')
    expect(tip).not.toMatch(/z-index:\s*\d/)
  })

  it('is what the "?" hint and the overlay mark in a key-value row use', () => {
    // Before: Hint kept its own popup with a z-index of 40, and the overlay mark had only a title, which a keyboard
    // cannot reach.
    expect(hint).toContain('<Tip>')
    expect(hint).not.toContain('role="tooltip"')
    expect(kvrows).toContain('<Tip text={marks[row.Key]}>')
  })
})
