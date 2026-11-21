import { describe, expect, it } from 'vitest'

describe('Open the example on the Manager screen', () => {
  it('should open the example through the bridge and then choose the demo environment', async () => {
    // Before: a new user had to find a server and a collection before any flow could run.
    const screen = (await import('./ManagerScreen.svelte?raw')).default as string

    expect(screen).toContain('ManagerService.OpenExample()')
    expect(screen).toContain("const EXAMPLE_ENVIRONMENT = 'demo'")
    expect(screen).toContain('app.selectEnvironment(EXAMPLE_ENVIRONMENT)')
    expect(screen).toContain('<Button variant="primary" loading={busy === \'example\'}')
  })
})
