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
    expect(popover).toContain('window.innerHeight - (drawn || height)')
    expect(popover).toContain('var(--z-menu)')
  })

  // Before: it was placed by the `height` it was told, so a taller one ran off the foot of the window, and a hint opened
  // in a card's panel was cut off at that panel's edge. It is placed by its real height and lives on <body>.
  it('is placed by its drawn height, never taller than the window, and lives on the page, not in the panel that opened it', () => {
    expect(popover).toContain('bind:offsetHeight={drawn}')
    expect(popover).toContain('calc(100vh - ${EDGE}px)')
    expect(popover).toContain('use:portal')
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
