import { describe, expect, it } from 'vitest'
import banner from '../Banner.svelte?raw'

const users = import.meta.glob('../{Announcements,EnvOffers,MissingBar,FileBanner,Busy}.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>
const flowEditor = (await import('../FlowEditor.svelte?raw')).default as string

describe('Banner', () => {
  it('announces a failure at once and everything else politely, in a tone that is also its colour', () => {
    expect(banner).toContain("role={tone === 'fail' ? 'alert' : 'status'}")
    expect(banner).toContain("tone?: 'info' | 'warn' | 'fail'")
    expect(banner).toContain('{@render actions()}')
  })

  it('is what the announcements, environment offer, missing-value bar, changed-on-disk bar, busy notice and run error use', () => {
    // Before: each wrote the strip's layout, tinted background and border, and some had no role.
    expect(Object.keys(users)).toHaveLength(5)
    for (const [path, text] of Object.entries(users)) {
      expect(text, path).toContain('<Banner')
      expect(text, path).not.toContain('role="status"')
    }
    expect(flowEditor).toContain('<Banner tone="fail">')
    expect(flowEditor).not.toContain('.runerr')
  })
})
