import { describe, expect, it } from 'vitest'

describe('the draft sweep', () => {
  it('should run one pass at a time and catch a failed one', async () => {
    // Before: TrashService.Sweep was called without await or catch, so a failure was an unhandled rejection and a slow
    // pass could overlap the next.
    const src = (await import('./app.svelte?raw')).default as string
    const at = src.indexOf('private sweepDrafts()')
    const body = src.slice(at, src.indexOf('onMenu(action', at))

    expect(body).toContain('if (running) return')
    expect(body).toContain('await TrashService.Sweep(this.tabRows)')
    expect(body).toMatch(/catch \{[^}]*\}\s*finally \{\s*running = false/)
  })
})
