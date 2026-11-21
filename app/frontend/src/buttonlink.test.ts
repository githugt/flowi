import { describe, expect, it } from 'vitest'
import button from './Button.svelte?raw'

const files = import.meta.glob('./{AssertCards,KVRows,PairsEditor,ValuesEditor,EnvEditor,RequestEditor,Inspector,HomeScreen,MethodPicker,OverlayDialog,SettingsScreen}.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

describe('Button link', () => {
  it('is words in the accent colour with no box, and underlined on hover', () => {
    expect(button).toContain("'link'")
    expect(button).toMatch(/\.link \{[^}]*color: var\(--accent\)/)
    expect(button).toContain('.link:hover:not(:disabled) { background: none; text-decoration: underline; }')
  })

  it('is what "+ Add", "Bulk edit" and the actions inside a line of text use, with no accent button of their own', () => {
    // Before: eleven files drew `.add` or `.link` as an accent-coloured button, each with its own padding, and some of
    // them kept the raised background of the file's other buttons.
    expect(Object.keys(files)).toHaveLength(11)
    for (const [path, text] of Object.entries(files)) {
      expect(text, path).toContain('variant="link"')
      expect((text.match(/<button[^>]*class="(add|link)"/g) ?? []).length, path).toBe(0)
    }
  })
})
