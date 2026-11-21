import { describe, expect, it } from 'vitest'
import empty from './EmptyState.svelte?raw'

const panes = import.meta.glob('./{Sidebar,SettingsScreen,TrafficLog,MessageFeed,StepsPanel}.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

describe('EmptyState kind', () => {
  it('has a filtered kind: the search icon, a title that names the filter, a Clear filter button and a status role', () => {
    // Before: a filter that hid everything left a plain muted line ("Nothing matches"), with no way to undo it.
    expect(empty).toContain("kind?: 'empty' | 'filtered'")
    expect(empty).toContain("'search'")
    expect(empty).toContain('Nothing matches “${query}”')
    expect(empty).toContain('Clear filter')
    expect(empty).toContain("role={kind === 'filtered' ? 'status' : undefined}")
  })

  it('is used by the five panes that filter a list, each clearing what it filters by', () => {
    expect(Object.keys(panes)).toHaveLength(5)
    for (const [path, text] of Object.entries(panes)) {
      expect(text, path).toMatch(/<EmptyState kind="filtered"[^>]*onclear=/)
      expect(text, path).not.toMatch(/<p class="(muted )?(none|note)">[^<]*(No|Nothing)[^<]*match/)
    }
    expect(panes['./TrafficLog.svelte']).toContain("mode = 'all'")
    expect(panes['./MessageFeed.svelte']).toContain("only = 'all'")
    expect(panes['./SettingsScreen.svelte']).toContain('onlyModified = false')
  })
})
