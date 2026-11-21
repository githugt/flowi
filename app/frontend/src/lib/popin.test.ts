import { describe, expect, it } from 'vitest'
// The test runner leaves a ?raw CSS import empty, so the file is read from disk.
// @ts-ignore
const { readFileSync } = (await import('node:fs')) as { readFileSync: (url: URL, enc: string) => string }
const css = readFileSync(new URL('../app.css', import.meta.url), 'utf8')
import menu from '../Menu.svelte?raw'
import popover from '../Popover.svelte?raw'

describe('menus and popovers', () => {
  it('share one rise-and-fade animation defined in app.css', () => {
    // Before: about 17 menus and popovers appeared and vanished with no motion.
    expect(css).toContain('@keyframes pop-in')
    expect(css).toContain('.pop-in { animation: pop-in var(--t-fast) var(--ease-out); }')
    expect(css).toContain('--ease-in:')
    for (const src of [menu, popover]) expect(src).toContain('pop-in')
  })
  it('stay still when the person asked for reduced motion', () => {
    expect(css).toMatch(/prefers-reduced-motion: reduce[\s\S]*animation-duration: 0\.01ms !important/)
  })
})
