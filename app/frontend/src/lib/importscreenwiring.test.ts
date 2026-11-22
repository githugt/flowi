import { describe, expect, it } from 'vitest'
import screen from '../ImportScreen.svelte?raw'
import offer from '../PasteOffer.svelte?raw'

describe('Import screen', () => {
  it('should not need a Preview button: the preview comes by itself and Refresh reads it again', () => {
    // Before: the first Preview was a primary button, and a pasted address only previewed after "More options…" then Preview.
    expect(screen).not.toMatch(/>\s*Preview\s*<\/Button>/)
    expect(screen).toContain('>Refresh</Button>')
    expect(screen).toContain('data-testid="preview-waiting"')
  })

  it('should take pasted text for every kind that is not a folder, with a clipboard button', () => {
    expect(screen).toContain("kind?.source !== 'folder'")
    expect(screen).toContain('navigator.clipboard.readText()')
  })

  it('should choose a folder as well as a collection', () => {
    expect(screen).toContain('ImportService.SetTarget(row)')
    expect(screen).not.toContain('collectionsOnly')
    expect(offer).toContain('ImportService.SetTarget(row)')
    expect(offer).not.toContain('collectionsOnly')
  })

  it('should say why there is no preview and what the last import did', () => {
    expect(screen).toContain('data-testid="preview-failed"')
    expect(screen).toContain('view.notes')
    expect(offer).toContain('preview.notes')
  })
})

describe('Paste offer', () => {
  it('should preview an offer that can only be imported without waiting for a click', () => {
    expect(offer).toContain('!canFill')
    expect(offer).toContain('untrack(importNew)')
  })

  it('should not ask for the preview itself after choosing the place: Go does it in order', () => {
    // Before: SetCollection and Preview were two calls that could arrive in the other order, which left "Reading it…" for good.
    expect(offer).not.toMatch(/function setCollection[\s\S]*ImportService\.Preview\(\)/)
  })
})
