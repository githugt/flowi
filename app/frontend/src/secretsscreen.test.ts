import { describe, expect, it } from 'vitest'

describe('the secrets screen', () => {
  it('should filter to missing secrets, list what uses a secret, and report a failed removal', async () => {
    // Before: the screen showed a name and a state; a failed Unset was dropped, and nothing said what used a secret.
    const src = (await import('./SecretsScreen.svelte?raw')).default as string

    expect(src).toContain('visibleSecrets(view.secrets, onlyMissing)')
    expect(src).toContain('usedByText(s.usedBy)')
    expect(src).toContain('reported(SecretsService.Unset(')
    expect(src).toContain("app.openScreen('screen:envcompare'")
    expect(src).toContain('{#each s.usedBy as u (u.kind + u.label)}')
  })

  it('should show whether the vault is open and let it be locked', async () => {
    // Before: nothing in the window said whether the vault was open, and nothing could lock it.
    const src = (await import('./SecretsScreen.svelte?raw')).default as string

    expect(src).toContain('vaultChip(view.vault)')
    expect(src).toContain('SecretsService.LockVault()')
    expect(src).toContain('disabled={!chip.canLock}')
  })
})
