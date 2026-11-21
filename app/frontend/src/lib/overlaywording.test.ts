import { describe, expect, it } from 'vitest'
import editor from '../RequestEditor.svelte?raw'
import specDialog from '../OverlayDialog.svelte?raw'

// Before: the Save menu offered "Save to flowi overlay…" and "Save to spec overlay…" side by side, two things that
// differ in one word. The second writes a file beside the API description, and says so now.
describe('the two overlays of the Save menu', () => {
  it('Given a request from a spec Then its menu names the spec overlay as a file of the API spec', () => {
    expect(editor).toContain("Save to the API spec's overlay file…")
    expect(editor).not.toContain('Save to spec overlay')
  })
  it('Given the dialog Then its title matches the menu item', () => {
    expect(specDialog).toContain('title="Save to the API spec\'s overlay file"')
  })
})
