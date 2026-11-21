import { describe, expect, it } from 'vitest'
import tabs from '../Tabs.svelte?raw'

const users = import.meta.glob('../{CodeView,Footer,ResponsePane}.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

describe('Tabs', () => {
  it('is a tablist with one tab stop, arrow keys, and a bare mode for a bar that sits in a row of its own', () => {
    expect(tabs).toContain('role="tablist"')
    expect(tabs).toContain('role="tab"')
    expect(tabs).toContain('tabKeyTarget')
    expect(tabs).toContain('bare')
    expect(tabs).toContain('after?: Snippet<[TabItem]>')
  })

  it('paints a count of errors as a failure, from the token', () => {
    expect(tabs).toContain("tone?: 'fail'")
    expect(tabs).toContain('background: var(--fail)')
  })

  it('is what the flow text, the bottom panel and the response pane use for their tabs', () => {
    // Before: each wrote its own buttons with `class:on` or `class:active`, its own underline and badge, and no tab
    // role or arrow keys.
    expect(Object.keys(users)).toHaveLength(3)
    for (const [path, text] of Object.entries(users)) {
      expect(text, path).toContain('<Tabs')
      expect(text, path).not.toContain('class:active={tab ===')
      expect(text, path).not.toMatch(/\.tabs\s/)
    }
  })

  it('keeps the bottom panel\'s trailing actions outside the tablist', () => {
    // The Refresh and Clear buttons and the version and location are not tabs: they sit beside the Tabs, not in it.
    const footer = Object.entries(users).find(([p]) => p.endsWith('Footer.svelte'))![1]
    expect(footer).toContain('<Tabs label="Panels" size="sm" bare')
    expect(footer).toMatch(/\/>\s*\{#if open === 'problems'\}/)
  })
})
