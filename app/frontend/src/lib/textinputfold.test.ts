import { describe, expect, it } from 'vitest'
import input from '../TextInput.svelte?raw'

const screens = import.meta.glob('../*.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

describe('TextInput is the one text field', () => {
  it('has no TextField left, in the code or on a screen', () => {
    // Before: TextField was a second component with its own height rule, a --bg background and its own focus colour,
    // and a label of its own that Field now draws.
    expect(Object.keys(screens).filter((p) => p.endsWith('/TextField.svelte'))).toEqual([])
    for (const [path, text] of Object.entries(screens)) expect(text, path).not.toMatch(/<TextField\b|TextField\.svelte/)
  })

  it('keeps the text being typed while Go answers, in reported mode, and hands the value to the parent in bound mode', () => {
    expect(input).toContain('typed ?? value')
    expect(input).toMatch(/if \(ontext\) \{\s*typed = text\s*ontext\(text\)\s*\} else value = text/)
    expect(input).toContain('typed = undefined')
  })

  it('has the extras a form field had: Enter, dots with an eye for a secret, and a warning about hidden characters', () => {
    expect(input).toContain('isEnter(e)')
    expect(input).toContain('onenter?.(shown)')
    expect(input).toContain('maskField(shown, secret, revealed)')
    expect(input).toContain('<HiddenChars')
    expect(input).toContain('height: var(--ctl-h)')
  })

  it('is given a label by Field, not by a prop of its own', () => {
    // A labelled field is <Field label=…><TextInput ontext=… /></Field>, so one label wrapper serves every control.
    const labelled = Object.entries(screens).filter(([, t]) => /<Field label=[^>]*><TextInput\b/.test(t))
    expect(labelled.length).toBeGreaterThan(3)
    for (const [path, text] of labelled) {
      for (const m of text.matchAll(/<Field label=[^>]*><TextInput\b([^>]*)\/>/g)) expect(m[1], path).toMatch(/ontext=|bind:value/)
    }
  })
})
