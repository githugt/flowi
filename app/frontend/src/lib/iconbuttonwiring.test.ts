import { describe, expect, it } from 'vitest'
import iconButton from '../IconButton.svelte?raw'

const users = import.meta.glob(
  '../{AssertCards,DatasetSections,EnvEditor,KVRows,PairsEditor,OutcomesEditor,RequestExtras,ValuesEditor,PartsEditor,FlowEditor,HomeScreen}.svelte',
  { query: '?raw', import: 'default', eager: true },
) as Record<string, string>

describe('IconButton', () => {
  it('always has a name for a screen reader, and shows a longer hint when there is one', () => {
    expect(iconButton).toContain('label: string')
    expect(iconButton).toContain('aria-label={label}')
    expect(iconButton).toContain('title={hint ?? label}')
  })

  it('is what the remove and dismiss buttons use, not a button with a × in it', () => {
    // Before: each row editor drew its own × as a text button with its own hover colour, size and focus look, and
    // most had no title.
    expect(Object.keys(users)).toHaveLength(11)
    for (const [path, text] of Object.entries(users)) {
      expect(text, path).toContain('<IconButton')
      expect(text, path).not.toMatch(/<button[^>]*>\s*×\s*<\/button/)
    }
  })
})
