import { describe, expect, it } from 'vitest'

describe('the checks pill of the response', () => {
  it('should sit in the response header and open the Assert section', async () => {
    // Before: a request with six passing checks never said "6/6"; the only trace was a tick on each card.
    const pane = (await import('../ResponsePane.svelte?raw')).default as string
    const editor = (await import('../RequestEditor.svelte?raw')).default as string

    expect(pane).toContain('checkTally(')
    expect(pane).toContain('open them')
    expect(editor).toContain("<ResponsePane {view} onchecks={() => (chosen = 'Assert')} />")
  })
})
