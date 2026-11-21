import { describe, expect, it } from 'vitest'

describe('the Runs tab baseline', () => {
  it('should compare a failed latest run with the last passed one, once for each latest run', async () => {
    // Before: "what changed since it last worked" meant picking two runs by hand.
    const src = (await import('./RunsTab.svelte?raw')).default as string

    expect(src).toContain('lastGreenIndex(runs.map((r) => r.status))')
    expect(src).toContain('autoFor !== latest')
    expect(src).toContain('void compare(green, 0)')
    expect(src).toContain('Compare with it')
  })
})
