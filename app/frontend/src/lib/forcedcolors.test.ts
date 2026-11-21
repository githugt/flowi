import { describe, expect, it } from 'vitest'

// The window follows forced colours and reduced transparency (the "System" setting). Before: neither was handled.
// @ts-ignore
const { readFileSync } = (await import('node:fs')) as { readFileSync: (url: URL, enc: string) => string }
const css = readFileSync(new URL('../app.css', import.meta.url), 'utf8')

describe('app.css', () => {
  it('has a forced-colors block that restores the focus ring and control edges', () => {
    const block = /@media \(forced-colors: active\) \{([^@]*)\}/.exec(css)?.[1] ?? ''

    expect(block).toContain(':focus-visible')
    expect(block).toMatch(/button[^{]*\{[^}]*border/)
  })
  it('drops the dialog blur for reduced transparency', () => {
    expect(css).toMatch(/@media \(prefers-reduced-transparency: reduce\)[^}]*backdrop-filter: none/)
  })
})

describe('canvas performance', () => {
  // Before: every card kept stacked drop-shadow filters while the canvas panned or zoomed, repainted on each frame.
  it('drops filters on the canvas while it moves', () => {
    expect(css).toMatch(/svg\.view-moving \* \{[^}]*filter: none !important/)
  })
})
