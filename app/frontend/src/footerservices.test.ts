import { describe, expect, it } from 'vitest'

describe('the footer service chips', () => {
  it('should come from the replayer and schedule events and open their screens', async () => {
    // Before: leaving the screen of a started replayer or schedule runner left nothing to say it was still working.
    const footer = (await import('./Footer.svelte?raw')).default as string
    const app = (await import('./lib/app.svelte?raw')).default as string

    expect(footer).toContain('serviceChips(app.services)')
    expect(footer).toContain('app.openScreen(s.screen')
    expect(footer).toContain('{#each services as s (s.id)}')
    expect(app).toContain("onEvent('replayer'")
    expect(app).toContain("onEvent('schedules'")
  })
})
