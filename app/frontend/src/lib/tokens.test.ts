import { describe, expect, it } from 'vitest'

// Colours live in app.css as theme tokens. A hex colour in a component would look
// right in one theme and wrong in the others, so none may come back.
const components = import.meta.glob('../*.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

describe('component styles', () => {
  it('use theme tokens, not hex colours', () => {
    const offenders = Object.entries(components).flatMap(([file, text]) =>
      [...text.matchAll(/#[0-9a-fA-F]{3}(?:[0-9a-fA-F]{3})?\b/g)].map((m) => `${file}: ${m[0]}`),
    )

    expect(Object.keys(components).length).toBeGreaterThan(20) // the glob still finds them
    expect(offenders).toEqual([])
  })
})
