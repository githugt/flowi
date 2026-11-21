import { describe, expect, it } from 'vitest'

describe('the changed-on-disk banner', () => {
  it('should let Keep mine allow the next save to overwrite the file', async () => {
    // Before: Keep mine only hid the banner, and a save wrote over a change made outside Flowi without any check.
    const banner = (await import('./FileBanner.svelte?raw')).default as string
    const editor = (await import('./FlowEditor.svelte?raw')).default as string

    expect(banner).toContain('kept = true; keep?.()')
    expect(editor).toContain('keep={() => FlowService.KeepMine(node)}')
  })
})
