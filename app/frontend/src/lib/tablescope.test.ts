import { describe, expect, it } from 'vitest'

// A header cell says whether it heads a column or a row, so a screen reader can read a cell with its headers.
// Before: no <th> had a scope, and the result grids had no caption.
const components = import.meta.glob('../*.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

describe('tables', () => {
  it('give every header cell a scope', () => {
    const bare = Object.entries(components)
      .flatMap(([file, text]) => text.split('\n').map((line, i) => ({ at: `${file.replace('../', '')}:${i + 1}`, line })))
      .filter(({ line }) => [...line.matchAll(/<th(\s[^>]*)?>/g)].some((m) => !/scope=/.test(m[0])))
      .map(({ at }) => at)

    expect(bare).toEqual([])
  })
})
