import { describe, expect, it } from 'vitest'
import segmented from '../Segmented.svelte?raw'

const users = import.meta.glob('../{FlowEditor,TrafficLog,MessageFeed,RequestEditor}.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

describe('Segmented', () => {
  it('is one radio group with one tab stop, named, with the chosen one marked', () => {
    expect(segmented).toContain('role="radiogroup"')
    expect(segmented).toContain('use:radiogroup')
    expect(segmented).toContain('aria-label={label}')
    expect(segmented).toContain('aria-checked={value === s.id}')
  })

  it('is what the view switch, the call filter, the message direction and the notes view use', () => {
    // Before: four hand-made copies with their own border, radius and selected colour; the message direction and the
    // notes view had no radio roles and no arrow keys.
    expect(Object.keys(users)).toHaveLength(4)
    for (const [path, text] of Object.entries(users)) {
      expect(text, path).toContain('<Segmented')
      expect(text, path).not.toMatch(/\.(seg|views|docsbar) button/)
    }
  })
})
