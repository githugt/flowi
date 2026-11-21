import { describe, expect, it } from 'vitest'
import button from '../Button.svelte?raw'
import menu from '../Menu.svelte?raw'
import empty from '../EmptyState.svelte?raw'
import splitter from '../Splitter.svelte?raw'

describe('the upgraded primitives', () => {
  it('lets Button be danger as a modifier on any look', () => {
    expect(button).toContain('danger?: boolean')
    expect(button).toContain('class:danger')
  })

  it('moves Menu from Home to End, hands the focus back on Escape, and sits on the menu layer', () => {
    // Before: only the up and down arrows moved, Escape left the focus on the page body, and the layer was 40.
    expect(menu).toContain("e.key === 'Home'")
    expect(menu).toContain("e.key === 'End'")
    expect(menu).toContain('opener.focus')
    expect(menu).toContain('z-index: var(--z-menu)')
    expect(menu).not.toMatch(/z-index:\s*\d/)
  })

  it('draws the buttons under an empty state as every Button, not restyled by it', () => {
    // Before: `.act :global(button)` gave every button inside an accent outline of its own.
    expect(empty).not.toContain(':global(button')
  })

  it('draws the splitter handle on a token layer', () => {
    expect(splitter).toContain('z-index: var(--z-handle)')
  })
})
