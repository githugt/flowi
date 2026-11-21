import { describe, expect, it } from 'vitest'
import { contrastRatio, inkForChosenAccent, readableInk } from './contrast'

// The accent of each theme is written in app.css, with its --on-accent (white when a theme gives none).
// The test runner leaves a ?raw CSS import empty, so the file is read from disk (the project has no Node typings).
// @ts-ignore
const { readFileSync } = (await import('node:fs')) as { readFileSync: (url: URL, enc: string) => string }
const sheet = readFileSync(new URL('../app.css', import.meta.url), 'utf8')

/** The themes that set --accent in one block: theme name -> its accent and the ink on it. */
function themeInks(): Record<string, { accent: string; ink: string }> {
  const out: Record<string, { accent: string; ink: string }> = {}
  for (const m of sheet.matchAll(/((?:\[data-theme='[^']+'\](?:,\s*)?)+)\s*\{([^}]*)\}/g)) {
    const accent = m[2].match(/--accent:\s*(#[0-9a-fA-F]{6})/)?.[1]
    if (!accent) continue
    const ink = m[2].match(/--on-accent:\s*(#[0-9a-fA-F]{3,6})/)?.[1] ?? '#fff'
    for (const t of m[1].matchAll(/\[data-theme='([^']+)'\]/g)) out[t[1]] = { accent, ink: expand(ink) }
  }
  return out
}
const expand = (h: string) => (h.length === 4 ? '#' + [...h.slice(1)].map((c) => c + c).join('') : h)

describe('contrastRatio', () => {
  it('is 21 for black on white and 1 for the same colour', () => {
    expect(contrastRatio('#000000', '#ffffff')).toBeCloseTo(21, 5)
    expect(contrastRatio('#336699', '#336699')).toBeCloseTo(1, 5)
  })
})

describe('readableInk', () => {
  it('picks the ink that reads better on the fill', () => {
    expect(readableInk('#4a63e0')).toBe('#ffffff')
    expect(readableInk('#88c0d0')).toBe('#10131a')
    expect(readableInk('#ffd700')).toBe('#10131a')
  })
})

describe('inkForChosenAccent', () => {
  it('gives the ink for the first accent that is a colour', () => {
    // Before: a custom accent always got white text, which fails on a light accent.
    expect(inkForChosenAccent('#88c0d0')).toBe('#10131a')
    expect(inkForChosenAccent('', '#1e66f5')).toBe('#ffffff')
    expect(inkForChosenAccent('#88c0d0', '#1e66f5')).toBe('#10131a')
  })
  it('gives nothing when the theme keeps its own accent', () => {
    expect(inkForChosenAccent('', undefined, 'red')).toBe('')
  })
})

describe('theme accents', () => {
  // Before: --on-accent was white in every theme, about 2.0 to 3.2:1 on the light accents of the dark themes.
  it('keep 4.5:1 or better for the ink on the accent in every theme', () => {
    const themes = themeInks()

    expect(Object.keys(themes).length).toBeGreaterThanOrEqual(8)
    for (const [name, { accent, ink }] of Object.entries(themes)) {
      expect(contrastRatio(accent, ink), name).toBeGreaterThanOrEqual(4.5)
    }
  })
})

describe('theme text', () => {
  // Each theme's text, muted text, accent and failure colours must be readable on its background and panel.
  it('meets 4.5:1 for text, muted text, accent and fail on bg and panel in every theme', () => {
    const vars = (body: string) => Object.fromEntries([...body.matchAll(/--(bg|panel|text|muted|accent|fail):\s*(#[0-9a-fA-F]{6})\b/g)].map((m) => [m[1], m[2]]))
    const base = vars(sheet.slice(sheet.indexOf(':root {'), sheet.indexOf('}', sheet.indexOf(':root {'))))
    const themes: Record<string, Record<string, string>> = {}
    for (const m of sheet.matchAll(/^((?:\[data-theme='[^']+'\](?:, )?)+) \{([^}]*)\}/gm)) {
      for (const n of m[1].matchAll(/data-theme='([^']+)'/g)) themes[n[1]] = { ...base, ...vars(m[2]) }
    }

    const low: string[] = []
    for (const [name, t] of Object.entries(themes)) {
      for (const fg of ['text', 'muted', 'accent', 'fail']) {
        for (const bg of ['bg', 'panel']) {
          const c = contrastRatio(t[fg], t[bg])
          if (c < 4.5) low.push(`${name}: ${fg} ${t[fg]} on ${bg} ${t[bg]} is ${c.toFixed(2)}:1`)
        }
      }
    }

    expect(Object.keys(themes).length).toBeGreaterThanOrEqual(8)
    // Before: two light themes had an accent under 4.5:1 on their panel.
    expect(low).toEqual([])
  })
})

describe('status and syntax colours', () => {
  // The dark set is the :root default over the dark bg and panel; the light set is data-mode='light' over the light theme's.
  // Before: only text, muted, accent and fail were checked, so a status or syntax colour could drop below 4.5:1 unseen.
  const hexes = (body: string) => Object.fromEntries([...body.matchAll(/--([a-z-]+):\s*(#[0-9a-fA-F]{6})\b/g)].map((m) => [m[1], m[2]]))
  const blockAfter = (marker: string) => {
    const at = sheet.indexOf(marker)
    return sheet.slice(at, sheet.indexOf('}', at))
  }
  const names = ['ok', 'warn', 'dirty', 'info', 'literal', 'pending', 'data', 'syn-key', 'syn-string', 'syn-number', 'syn-atom']

  function lowOn(colours: Record<string, string>, backgrounds: Record<string, string>): string[] {
    const low: string[] = []
    for (const n of names) {
      for (const [bgName, bg] of Object.entries(backgrounds)) {
        const c = contrastRatio(colours[n], bg)
        if (c < 4.5) low.push(`${n} ${colours[n]} on ${bgName} ${bg} is ${c.toFixed(2)}:1`)
      }
    }
    return low
  }

  it('reach 4.5:1 on the dark background and panel', () => {
    const colours = { ...hexes(sheet.slice(sheet.indexOf(':root {\n  --ok:'), sheet.indexOf('}', sheet.indexOf(':root {\n  --ok:')))), ...hexes(blockAfter(':root { --syn-key')) }

    expect(lowOn(colours, { bg: '#131519', panel: '#1a1d22' })).toEqual([])
  })

  it('reach 4.5:1 on the light background and panel', () => {
    const colours = { ...hexes(blockAfter(":root[data-mode='light'] {\n  --ok")), ...hexes(blockAfter(":root[data-mode='light'] {\n  --syn-key")) }

    expect(lowOn(colours, { bg: '#ffffff', panel: '#f7f8fa' })).toEqual([])
  })
})
