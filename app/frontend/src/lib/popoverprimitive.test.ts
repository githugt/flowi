import { describe, expect, it } from 'vitest'
import popover from '../Popover.svelte?raw'

const users = import.meta.glob('../{StepPopup,CaseEditor,IOEditor}.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

describe('Popover', () => {
  it('opens at a point over a transparent layer, closes on a press outside or Escape, and is named and trapped like a dialog', () => {
    expect(popover).toContain('onpointerdown={onclose}')
    expect(popover).toContain("e.key === 'Escape'")
    expect(popover).toContain('use:modal')
    expect(popover).toContain('role="dialog"')
    expect(popover).toContain('aria-label={label}')
  })

  it('keeps itself inside the window and takes its layers from the z-index tokens', () => {
    expect(popover).toContain('window.innerWidth - width')
    expect(popover).toContain('window.innerHeight - height')
    expect(popover).toContain('var(--z-menu)')
  })

  it('is what the step picker, a Route test and a flow input or output use, with no layer or box of their own', () => {
    // Before: each wrote `.scrim` (z-index 29), the box (z-index 30) and its own clamp to the window edge.
    expect(Object.keys(users)).toHaveLength(3)
    for (const [path, text] of Object.entries(users)) {
      expect(text, path).toContain('<Popover')
      expect(text, path).not.toMatch(/\.scrim\s*\{/)
      expect(text, path).not.toContain('use:modal')
      expect(text, path).not.toContain('window.innerWidth')
    }
  })
})
