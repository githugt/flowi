import { describe, expect, it } from 'vitest'

describe('the About screen', () => {
  it('should offer the diagnostics for a bug report and a way to the workspace folder', async () => {
    // Before: it showed a version and nothing else a report needs.
    const src = (await import('../AboutScreen.svelte?raw')).default as string

    expect(src).toContain('SystemService.Diagnostics()')
    expect(src).toContain('Copy diagnostics')
    expect(src).toContain('WorkspaceService.RevealWorkspace()')
    expect(src).toContain('disabled={!app.tree?.folder}')
  })
})
