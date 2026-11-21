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

describe('type scale', () => {
  // The test runner leaves a ?raw CSS import empty, so the file is read from disk (the project has no Node typings).
  it('follows the font size setting: every --fs-* token is built from --font-size', async () => {
    // @ts-ignore
    const { readFileSync } = (await import('node:fs')) as { readFileSync: (url: URL, enc: string) => string }
    const sheet = readFileSync(new URL('../app.css', import.meta.url), 'utf8')

    const scale = [...sheet.matchAll(/(--fs-(?:xs|sm|md|lg|xl)):\s*([^;]+);/g)]

    expect(scale.map((m) => m[1])).toEqual(['--fs-xs', '--fs-sm', '--fs-md', '--fs-lg', '--fs-xl'])
    // Before: the scale was fixed pixels (11, 12, 13, 15, 20), so the setting changed only the body text.
    for (const [, name, value] of scale) expect(value, name).toContain('var(--font-size')
  })
})

describe('reduced motion', () => {
  it('stops every looping animation, not only the toast and the dialog', async () => {
    // @ts-ignore
    const { readFileSync } = (await import('node:fs')) as { readFileSync: (url: URL, enc: string) => string }
    const sheet = readFileSync(new URL('../app.css', import.meta.url), 'utf8')
    const block = sheet.slice(sheet.indexOf('@media (prefers-reduced-motion: reduce)'))

    // Before: only Toasts.svelte and Overlays.svelte had a reduced-motion rule; the canvas arcs looped for ever.
    expect(block).toMatch(/animation-iteration-count:\s*1\s*!important/)
    expect(block).toMatch(/\*, \*::before, \*::after/)
  })
})

describe('native controls', () => {
  it('take the accent colour', async () => {
    // @ts-ignore
    const { readFileSync } = (await import('node:fs')) as { readFileSync: (url: URL, enc: string) => string }
    const sheet = readFileSync(new URL('../app.css', import.meta.url), 'utf8')

    // Before: checkboxes and radios drew in the browser's blue whatever the theme.
    expect(sheet).toMatch(/:root\s*\{\s*accent-color:\s*var\(--accent\)/)
  })
})

describe('system settings', () => {
  it('follows more-contrast under "system" and has thin themed scrollbars', async () => {
    // @ts-ignore
    const { readFileSync } = (await import('node:fs')) as { readFileSync: (url: URL, enc: string) => string }
    const sheet = readFileSync(new URL('../app.css', import.meta.url), 'utf8')

    // Before: neither existed; the High contrast theme had to be chosen by hand and the scrollbars were the system's wide ones.
    expect(sheet).toMatch(/@media \(prefers-contrast: more\)\s*\{\s*:root:not\(\[data-theme\]\)/)
    expect(sheet).toMatch(/scrollbar-width:\s*thin/)
    expect(sheet).toMatch(/::-webkit-scrollbar-thumb/)
  })
})

describe('custom properties', () => {
  // A var(--name) that nothing defines is silently empty: the rule it sits in does nothing, in every theme.
  it('every var(--name) used in a component or lib file is defined somewhere', () => {
    const files = {
      ...(import.meta.glob('../*.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>),
      ...(import.meta.glob('./*.ts', { query: '?raw', import: 'default', eager: true }) as Record<string, string>),
    }
    // The theme sheet is read from disk (a ?raw CSS import is empty under the test runner).
    const defined = new Set<string>()
    const used: { file: string; name: string }[] = []
    const add = (text: string, file: string, sheet: boolean) => {
      for (const m of text.matchAll(/(--[a-zA-Z0-9-]+)\s*:/g)) defined.add(m[1])
      for (const m of text.matchAll(/style:(--[a-zA-Z0-9-]+)/g)) defined.add(m[1])
      for (const m of text.matchAll(/['"`](--[a-zA-Z0-9-]+)['"`]/g)) defined.add(m[1])
      if (sheet) return
      for (const m of text.matchAll(/var\((--[a-zA-Z0-9-]+)/g)) if (!m[1].endsWith('-')) used.push({ file, name: m[1] })
    }
    return (async () => {
      // @ts-ignore
      const { readFileSync } = (await import('node:fs')) as { readFileSync: (url: URL, enc: string) => string }
      add(readFileSync(new URL('../app.css', import.meta.url), 'utf8'), 'app.css', true)
      for (const [file, text] of Object.entries(files)) if (!file.endsWith('.test.ts')) add(text, file, false)

      // Before: FlowiOverlayEditor and MessageFeed used var(--hover), which no theme defines, so their row hover did nothing.
      const missing = used.filter((u) => !defined.has(u.name)).map((u) => `${u.file}: ${u.name}`)
      expect(missing).toEqual([])
    })()
  })
})

// A ratchet, like the raw-<button> count in designsystem.test.ts: these literals are being moved to tokens (--fs-*,
// --r-*, --mono), so each count may go down and must never go up. When one drops, lower the number here.
// Before: the check covered hex colours only, so pixel radii, pixel font sizes and monospace stacks crept back.
describe('literal sizes and fonts in components', () => {
  const count = (re: RegExp) => Object.values(components).reduce((n, text) => n + [...text.matchAll(re)].length, 0)

  it('has no more pixel font sizes than the current baseline', () => {
    expect(count(/font-size:\s*[0-9.]+px/g)).toBeLessThanOrEqual(12)
  })

  it('has no more pixel border radii than the current baseline', () => {
    expect(count(/border-radius:\s*[0-9.]+px/g)).toBeLessThanOrEqual(29)
  })

  it('has no more literal monospace stacks than the current baseline', () => {
    expect(count(/font(?:-family)?:[^;}]*\b(?:ui-monospace|Menlo|monospace)\b/g)).toBeLessThanOrEqual(27)
  })
})
