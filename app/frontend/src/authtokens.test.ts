import { describe, expect, it } from 'vitest'

describe('the saved sign-ins on the Authentications screen', () => {
  it('should list the tokens without their values, confirm before forgetting one and reload after', async () => {
    // Before: a sign-in a person approved could be neither seen nor dropped from the window.
    const src = (await import('./AuthsScreen.svelte?raw')).default as string

    expect(src).toContain('await AuthsService.Tokens()')
    expect(src).toContain("ui.confirm('Forget this sign-in?'")
    expect(src).toMatch(/await reported\(AuthsService\.ForgetToken\(t\.id\), ui\.toast\)\) await loadTokens\(\)/)
    expect(src).toMatch(/catch \{\s*tokens = \[\]/)
    expect(src).not.toMatch(/access|refresh_token/i)
  })
})
