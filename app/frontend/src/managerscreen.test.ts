import { describe, expect, it } from 'vitest'

describe('the collections screen actions', () => {
  it('should keep what was typed until the action worked', async () => {
    // Before: Mount, Open, Create and the rest cleared their fields at once, so a refusal meant typing it all again.
    const src = (await import('./ManagerScreen.svelte?raw')).default as string

    for (const call of ['ManagerService.Mount(mountDir)', 'ManagerService.NewCollection(newName, newLocation)', 'ManagerService.OpenArazzo(arazzoRef)', 'ManagerService.OpenBundle(bundleFile)', 'ManagerService.Relocate(c.id, relocate[c.id])', 'ManagerService.SaveDraft(c.id, draftDir[c.id])']) {
      expect(src).toContain(`attempt(`)
      expect(src).toContain(call)
    }
    expect(src).toContain('if (await reported(call, ui.toast, ok)) clear()')
    expect(src).not.toMatch(/\{ ManagerService\.(Mount|OpenArazzo|OpenBundle|NewCollection)\([^)]*\); /)
  })
})
