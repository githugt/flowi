import { describe, expect, it } from 'vitest'
import editor from '../RequestEditor.svelte?raw'

describe('the Send button', () => {
  it('goes through one dispatch that ignores a second click until Go has answered', () => {
    // Before: the button waited for Go to report "sending", so a quick second click sent the request twice.
    expect(editor).toContain('if (pending) return')
    expect(editor).toContain('.finally(() => (pending = false))')
  })
  it('is disabled and busy while the click is pending, also after a protected-environment confirm', () => {
    expect(editor).toContain('disabled={pending} aria-busy={pending}')
    expect(editor).toContain('dispatch, \'Send\')')
    expect(editor.match(/RequestService\.Send\(node/g)?.length).toBe(1)
  })
})
