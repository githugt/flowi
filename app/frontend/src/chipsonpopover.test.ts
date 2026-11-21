import { describe, expect, it } from 'vitest'
import popover from './Popover.svelte?raw'

const callers = import.meta.glob('./{Chips,SkipChip,VarsBar,HiddenChars}.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

describe('chips and the hidden-characters warning on Popover', () => {
  it('opens its panel with Popover from the button\'s edge, and no longer closes it from a window pointer handler', () => {
    // Before: each wrote an absolutely placed .pop, stopped the pointer-down of its button and of its panel, and closed
    // on a window pointer-down handler, so none had Escape, a focus trap or a clamp to the window edge.
    expect(Object.keys(callers)).toHaveLength(4)
    for (const [path, text] of Object.entries(callers)) {
      expect(text, path).toMatch(/<Popover /)
      expect(text, path).not.toMatch(/<svelte:window[^>]*onpointerdown/)
      expect(text, path).not.toContain('stopPropagation')
      expect(text, path).toContain('getBoundingClientRect()')
      expect(text, path).not.toMatch(/\.pop \{/)
    }
  })

  it('opens upward for the bar at the foot of the flow, and right-aligned for the button at the end of a field', () => {
    expect(popover).toContain('above?: boolean')
    expect(popover).toContain('end?: boolean')
    expect(popover).toContain('style:bottom={above ?')
    expect(callers['./VarsBar.svelte']).toMatch(/<Popover [^>]*\babove\b/)
    expect(callers['./HiddenChars.svelte']).toMatch(/<Popover [^>]*\bend\b/)
  })
})
