import { describe, expect, it } from 'vitest'
import kv from '../KVRows.svelte?raw'
import parts from '../Parts.svelte?raw'
import env from '../EnvEditor.svelte?raw'
import state from './app.svelte.ts?raw'
// The test runner leaves a ?raw CSS import empty, so the file is read from disk (the project has no Node typings).
// @ts-ignore
const { readFileSync } = (await import('node:fs')) as { readFileSync: (url: URL, enc: string) => string }
const css = readFileSync(new URL('../app.css', import.meta.url), 'utf8')

// The blur is only a screen filter: these check the wiring from the setting to the cells that carry credentials.
describe('blur secrets on screen', () => {
  it('Given the setting Then the root carries data-hide-sensitive and the CSS blurs .sensitive', () => {
    expect(state).toContain("root.dataset.hideSensitive = 'true'")
    expect(css).toMatch(/\[data-hide-sensitive='true'\] \.sensitive \{ filter: blur/)
  })
  it('Given header, parameter and variable cells Then each marks its credentials', () => {
    expect(kv).toContain('class:sensitive={sensitiveValue(row.Key, row.Value)}')
    expect(parts).toContain('class:sensitive={sensitiveValue(r.key, r.value)}')
    expect(env).toContain('<span class="cell sensitive">')
  })
})
