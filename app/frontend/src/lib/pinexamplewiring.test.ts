import { describe, expect, it } from 'vitest'
import pane from '../ResponsePane.svelte?raw'
import stored from '../StoredResponses.svelte?raw'
import overlay from '../FlowiOverlayEditor.svelte?raw'

describe('pin and overlay delete wiring', () => {
  it('should ask before pinning over an example of the same name, from both places that pin', () => {
    // Before: pinning wrote over an example of the same name without a word.
    expect(pane).toContain("RequestService.ExampleTaken(node, '', '', name)")
    expect(stored).toContain('RequestService.ExampleTaken(node, r.version, r.name, name)')
    expect(pane).toContain('ui.confirm(d.title, d.text, go, d.action)')
    expect(stored).toContain('ui.confirm(d.title, d.text, go, d.action)')
  })

  it('should name the flows that use an overlay in the delete confirm', () => {
    expect(overlay).toContain('RequestService.FlowiOverlayUses(node, name)')
    expect(overlay).toContain('overlayDeleteText(name, uses)')
  })
})
