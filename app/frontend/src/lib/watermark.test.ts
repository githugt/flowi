import { describe, expect, it } from 'vitest'
import { watermarkHints } from './watermark'

describe('watermarkHints', () => {
  it('writes the keys for the platform', () => {
    // Before: the empty area named no shortcut.
    expect(watermarkHints(false).map((h) => h.keys)).toEqual(['Ctrl+N', 'Ctrl+K', 'Ctrl+Shift+P', 'Ctrl+I'])
    expect(watermarkHints(true).map((h) => h.keys)).toEqual(['⌘N', '⌘K', '⌘⇧P', '⌘I'])
  })
  it('names what each does', () => {
    expect(watermarkHints(false).map((h) => h.label)).toContain('Command palette')
  })
})
