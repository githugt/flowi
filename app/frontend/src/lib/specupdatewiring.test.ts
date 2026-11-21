import { describe, expect, it } from 'vitest'
import dialog from '../SpecUpdateDialog.svelte?raw'

describe('Update from spec dialog wiring', () => {
  it('should apply the source that was previewed and show what breaks above the button', () => {
    // Before: Apply took only the spec's name, and the breakages were in another screen.
    expect(dialog).toContain('ApplySpecUpdate(collection, spec.name, preview?.source ?? \'\')')
    expect(dialog).toContain('breakHeading(preview)')
    expect(dialog.indexOf('breakHeading(preview)')).toBeLessThan(dialog.indexOf('updateLabel(preview)'))
  })
})
