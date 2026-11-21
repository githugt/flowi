import { describe, expect, it } from 'vitest'

// What floats over the page (menus, dialogs, toasts, drag marks) takes its layer from the --z-* tokens in app.css.
// Before: sixteen different numbers were spelled in the components, and a dialog over a dialog had to guess one.
// @ts-ignore
const { readFileSync } = (await import('node:fs')) as { readFileSync: (url: URL, enc: string) => string }
const css = readFileSync(new URL('../app.css', import.meta.url), 'utf8')
const components = import.meta.glob('../*.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

describe('layers', () => {
  it('are named in every component: no number of 50 or more is written next to z-index', () => {
    const raw = Object.entries(components)
      .flatMap(([file, text]) => text.split('\n').map((line, i) => ({ at: `${file.replace('../', '')}:${i + 1}`, line })))
      .filter(({ line }) => /z-index:\s*([5-9]\d|\d{3,})\b/.test(line))
      .map(({ at }) => at)

    expect(raw).toEqual([])
  })

  it('are defined once, in rising order', () => {
    const text = css
    const value = (name: string) => Number(new RegExp(`--${name}: (\\d+)`).exec(text)?.[1])
    const order = ['z-menu', 'z-modal', 'z-modal-above', 'z-top', 'z-drag-mark', 'z-drag'].map(value)

    expect(order.every((n) => Number.isFinite(n))).toBe(true)
    expect([...order].sort((a, b) => a - b)).toEqual(order)
    expect(new Set(order).size).toBe(order.length)
  })
})
