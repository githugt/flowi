import { describe, expect, it } from 'vitest'
import tabs from '../Tabs.svelte?raw'
import sidepanel from '../SidePanel.svelte?raw'
import dock from '../OutputDock.svelte?raw'
import settings from '../SettingsScreen.svelte?raw'
import help from '../ExprHelp.svelte?raw'
import { tabKeyTarget } from './tabkeys'

describe('tabKeyTarget', () => {
  it('Given Left and Right When pressed Then focus moves and wraps round', () => {
    // Before: the tab bars had no arrow keys, so every tab was a tab stop.
    expect(tabKeyTarget('ArrowRight', 0, 3)).toBe(1)
    expect(tabKeyTarget('ArrowRight', 2, 3)).toBe(0)
    expect(tabKeyTarget('ArrowLeft', 0, 3)).toBe(2)
  })
  it('Given Home and End When pressed Then focus goes to the ends', () => {
    expect(tabKeyTarget('Home', 2, 3)).toBe(0)
    expect(tabKeyTarget('End', 0, 3)).toBe(2)
  })
  it('Given Up, Down or another key, or focus outside the tabs Then the page keeps it', () => {
    expect(tabKeyTarget('ArrowDown', 0, 3)).toBeNull()
    expect(tabKeyTarget('ArrowUp', 1, 3)).toBeNull()
    expect(tabKeyTarget('a', 0, 3)).toBeNull()
    expect(tabKeyTarget('ArrowRight', -1, 3)).toBeNull()
  })
})

describe('Tabs component', () => {
  it('Given the bar When drawn Then it has the roles, one tab stop and the keys', () => {
    expect(tabs).toContain('role="tablist"')
    expect(tabs).toContain('role="tab"')
    expect(tabs).toContain('aria-selected={active === t.id}')
    expect(tabs).toContain('tabindex={t.id === stop ? 0 : -1}')
    expect(tabs).toContain('tabKeyTarget(')
  })
  it('Given the screens that had their own bars When read Then they use the component', () => {
    for (const src of [sidepanel, dock, settings, help]) {
      expect(src).toContain('<Tabs ')
      expect(src).not.toContain('role="tablist"')
    }
  })
})
