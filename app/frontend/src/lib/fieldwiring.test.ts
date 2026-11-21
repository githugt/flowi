import { describe, expect, it } from 'vitest'
import field from '../Field.svelte?raw'

const users = import.meta.glob('../{ImportScreen,PasteOffer,PlaceDialog,MessageSections,ReplayerScreen,IOEditor,ParamFields,Inspector,WirePanel,RowEditor}.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

describe('Field', () => {
  it('is a label around the words and one control, so a click on the words focuses it', () => {
    expect(field).toContain('<label class="field"')
    expect(field).toContain('{label}')
    expect(field).toContain('{@render children()}')
    expect(field).toContain('grid-column: 1 / -1')
  })

  it('is what the import, replayer, place, paste, wire and input-output forms use for a label over a control', () => {
    // Before: each wrote `<label class="f"><span>…</span>` or `.field` and `.label`, with its own copy of the CSS.
    expect(Object.keys(users)).toHaveLength(10)
    for (const [path, text] of Object.entries(users)) {
      expect(text, path).toContain('<Field ')
      expect(text, path).not.toMatch(/<label class="f(ield)?(?: wide)?">\s*<span(?: class="label")?>/)
    }
  })
})
