import { describe, expect, it } from 'vitest'
import { radioKeyTarget, tabStop } from './radiogroup'

const screens = import.meta.glob(['../OutcomesEditor.svelte', '../EnvEditor.svelte', '../SettingsScreen.svelte', '../Segmented.svelte'], { query: '?raw', import: 'default', eager: true }) as Record<string, string>

describe('radio groups', () => {
  it('moves with the arrows, wrapping round, and Home and End jump', () => {
    expect(radioKeyTarget('ArrowRight', 0, 4)).toBe(1)
    expect(radioKeyTarget('ArrowDown', 3, 4)).toBe(0)
    expect(radioKeyTarget('ArrowLeft', 0, 4)).toBe(3)
    expect(radioKeyTarget('ArrowUp', 2, 4)).toBe(1)
    expect(radioKeyTarget('Home', 2, 4)).toBe(0)
    expect(radioKeyTarget('End', 0, 4)).toBe(3)
    expect(radioKeyTarget('a', 0, 4)).toBeNull()
    expect(radioKeyTarget('ArrowRight', 0, 0)).toBeNull()
  })

  it('makes the chosen radio the tab stop, else the first', () => {
    // Before: every swatch was a tab stop.
    expect(tabStop([false, false, true, false])).toBe(2)
    expect(tabStop([false, false])).toBe(0)
    expect(tabStop([])).toBe(0)
  })

  it('is on every radiogroup in the screens', () => {
    for (const [file, text] of Object.entries(screens)) {
      const groups = text.match(/<div [^>]*role="radiogroup"[^>]*>/g) ?? []

      expect(groups.length, file).toBeGreaterThan(0)
      for (const g of groups) expect(g, file).toContain('use:radiogroup')
    }
  })
})
