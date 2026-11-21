import { describe, expect, it } from 'vitest'
import screen from '../AuthsScreen.svelte?raw'

describe('Auths screen wiring', () => {
  it('should wait for each call and show how it went, keeping the typed name when making one fails', () => {
    // Before: saves were sent and forgotten: a refusal came as a notice elsewhere and the form was already cleared.
    expect(screen).toContain('reported(AuthsService.Create(into, name, scheme), ui.toast)')
    expect(screen).toContain('reported(AuthsService.Rename(a.key, name), ui.toast')
    expect(screen).toContain('reported(AuthsService.Delete(a.key), ui.toast')
    expect(screen.indexOf('reported(AuthsService.Create')).toBeLessThan(screen.indexOf("name = ''"))
  })

  it('should ask before closing or replacing a form that holds unsaved edits', () => {
    expect(screen).toContain('ui.unsaved(')
    expect(screen).toContain('authDirty(a, draft)')
    expect(screen).toContain('Unsaved changes')
  })
})
