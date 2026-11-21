import { describe, expect, it } from 'vitest'
import page from '../ScreenPage.svelte?raw'

const users = import.meta.glob('../{Blueprints,Checks,EnvCompare,Manager,Schedules,Secrets,Telemetry,Tracing,Trash,Import}Screen.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

describe('ScreenPage', () => {
  it('fills the tab, scrolls by itself, and keeps its content to a width', () => {
    expect(page).toContain('position: absolute; inset: 0; overflow: auto')
    expect(page).toContain('style:max-width')
  })

  it('is the page of the blueprints, checks, environment comparison, manager, schedules, secrets, telemetry, tracing, trash and import screens', () => {
    // Before: each wrote `.page` with the same padding and its own max-width.
    expect(Object.keys(users)).toHaveLength(10)
    for (const [path, text] of Object.entries(users)) {
      expect(text, path).toContain('<ScreenPage')
      expect(text, path).not.toMatch(/^  \.page \{/m)
    }
  })
})
