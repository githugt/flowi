import { describe, expect, it } from 'vitest'
import { settingAnnouncement } from './settingsaid'

const screen = import.meta.glob('../SettingsScreen.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

describe('settingAnnouncement', () => {
  const before = [{ name: 'appearance.theme', value: 'dark' }, { name: 'http.timeout', value: '30s' }]

  it('says Saved for a value that changed', () => {
    const after = [{ name: 'appearance.theme', value: 'light' }, { name: 'http.timeout', value: '30s' }]

    expect(settingAnnouncement(before, after)).toBe('Saved: Theme')
  })

  it('says Not saved with the reason when a value was refused', () => {
    // Before: the refusal was a red line only.
    const after = [{ name: 'appearance.theme', value: 'dark' }, { name: 'http.timeout', value: '30s', problem: 'not a duration' }]

    expect(settingAnnouncement(before, after)).toMatch(/^Not saved: .*: not a duration$/)
  })

  it('says nothing for the first view or for no change, and does not repeat a standing problem', () => {
    const bad = [{ name: 'http.timeout', value: '30s', problem: 'not a duration' }]

    expect(settingAnnouncement([], before)).toBe('')
    expect(settingAnnouncement(before, before)).toBe('')
    expect(settingAnnouncement(bad, bad)).toBe('')
  })

  it('is wired as a polite live region with the field described by its problem', () => {
    const text = Object.values(screen)[0]

    expect(text).toContain('aria-live="polite"')
    expect(text).toContain('settingAnnouncement(')
    expect(text).toContain('describedby=')
  })
})
