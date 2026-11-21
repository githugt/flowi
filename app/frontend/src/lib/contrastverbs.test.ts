import { describe, expect, it } from 'vitest'
import { contrastRatio } from './contrast'

// The test runner leaves a ?raw CSS import empty, so the file is read from disk (the project has no Node typings).
// @ts-ignore
const { readFileSync } = (await import('node:fs')) as { readFileSync: (url: URL, enc: string) => string }
const sheet = readFileSync(new URL('../app.css', import.meta.url), 'utf8')

const hexes = (line: string, prefix: string) => Object.fromEntries([...line.matchAll(new RegExp(`--${prefix}-([A-Za-z]+):\\s*(#[0-9a-fA-F]{6})\\b`, 'g'))].map((m) => [m[1], m[2]]))
const lineWith = (start: string) => sheet.split('\n').find((l) => l.startsWith(start)) ?? ''

describe('verb colours', () => {
  // Before: only text, muted, accent, fail, status and syntax colours were checked; a verb (GET, SQL…) is drawn as text
  // on the panel, and the light SQL colour was 4.41:1 there.
  const dark = hexes(lineWith(':root { --v-GET'), 'v')
  const light = hexes(lineWith(":root[data-mode='light'] { --v-GET"), 'v')

  it('are read from both sets', () => {
    expect(Object.keys(dark).length).toBeGreaterThan(20)
    expect(Object.keys(light)).toEqual(Object.keys(dark))
  })

  it('reach 4.5:1 on the dark background and panel', () => {
    const low = Object.entries(dark).flatMap(([n, c]) => ['#131519', '#1a1d22'].filter((bg) => contrastRatio(c, bg) < 4.5).map((bg) => `${n} ${c} on ${bg}`))

    expect(low).toEqual([])
  })

  it('reach 4.5:1 on the light background and panel', () => {
    const low = Object.entries(light).flatMap(([n, c]) => ['#ffffff', '#f7f8fa'].filter((bg) => contrastRatio(c, bg) < 4.5).map((bg) => `${n} ${c} on ${bg}`))

    expect(low).toEqual([])
  })
})

describe('the Send button', () => {
  // The label is white (--on-send) on a gradient between two stops per colour choice. Under 4.5:1 is a known gap in
  // the stock colours (a bold 13 px label is not "large text"); the list may not grow, and nothing may fall under 3:1.
  const stops = (block: string) => [...block.matchAll(/--send-([ab]):\s*(#[0-9a-fA-F]{6})/g)].map((m) => m[2])
  const choices: Record<string, string[]> = {
    default: stops(lineWith('  --send-a: #')),
    violet: stops(lineWith("[data-send='violet']")),
    emerald: stops(lineWith("[data-send='emerald']")),
    cyan: stops(lineWith("[data-send='cyan']")),
  }
  const KNOWN_UNDER_4_5 = ['default #d4541a', 'violet #5a78ee', 'violet #8a63d8', 'emerald #1f9e70', 'cyan #0f8fb0']

  it('reads every colour choice from app.css', () => {
    for (const [name, s] of Object.entries(choices)) expect(s, name).toHaveLength(2)
  })

  it('keeps white on every stop at 3:1 or better, and the stops under 4.5:1 to the known ones', () => {
    const under3: string[] = []
    const under45: string[] = []
    for (const [name, s] of Object.entries(choices)) {
      for (const c of s) {
        const r = contrastRatio('#ffffff', c)
        if (r < 3) under3.push(`${name} ${c} ${r.toFixed(2)}`)
        if (r < 4.5) under45.push(`${name} ${c}`)
      }
    }

    expect(under3).toEqual([])
    expect(under45.filter((x) => !KNOWN_UNDER_4_5.includes(x))).toEqual([])
  })
})
