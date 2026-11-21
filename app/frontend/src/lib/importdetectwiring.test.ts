import { describe, expect, it } from 'vitest'
import screen from '../ImportScreen.svelte?raw'

describe('Import screen detection note', () => {
  it('should say what the source was recognised as, so the change of kind is not a surprise', () => {
    // Before: the kind stayed as it was and a wrong one failed only at the preview.
    expect(screen).toContain('{#if view.detected}')
    expect(screen).toContain('Recognised {view.detected}')
  })
})
