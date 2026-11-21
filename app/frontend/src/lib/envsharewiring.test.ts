import { describe, expect, it } from 'vitest'
import editor from '../EnvEditor.svelte?raw'

describe('share environment wiring', () => {
  it('should offer Share with… beside Save, disabled while there are unsaved edits', () => {
    // Before: environments could be imported from a collection but never exported to one.
    expect(editor).toContain('>Share with…</Button>')
    expect(editor).toContain('disabled={view.dirty || !view.onDisk}')
  })

  it('should list the mounted collections and ask before replacing a file', () => {
    expect(editor).toContain('EnvironmentService.ShareTargets()')
    expect(editor).toContain('EnvironmentService.Share(node, collection, overwrite)')
    expect(editor).toContain('ui.confirm(d.title, d.text, () => share(collection, label, true), d.action)')
  })
})
