import { describe, expect, it } from 'vitest'

describe('Problem with a blocked secret', () => {
  it('should offer Allow secrets to each host through the manager bridge, with the design-system Button', async () => {
    // Before: the only ways to trust a host were the CLI and the Settings list.
    const problem = (await import('./Problem.svelte?raw')).default as string

    expect(problem).toContain('ManagerService.AllowSecretHost(host)')
    expect(problem).toContain('<Button small variant="ghost" loading={allowing === host}')
    expect(problem).not.toMatch(/<button\b/)
  })
})
