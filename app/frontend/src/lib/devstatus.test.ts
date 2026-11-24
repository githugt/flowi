import { describe, expect, it } from 'vitest'
import { cleanStatusName, cleanTag, cleanTags, hasLabelTerms, labelChoices, toggleTerm, labelsMatch, labelText, parseQuery, statusColor, statusTitle, statusWords, suggestTags } from './devstatus'
import { contrastRatio } from './contrast'
import { ICONS } from './icons'

const mark = (name: string, extra = {}) => ({ name, color: 'blue', icon: 'code', ...extra })

describe('parseQuery', () => {
  it('splits words from tag and status terms', () => {
    expect(parseQuery('login tag:Smoke #auth status:Review')).toMatchObject({ words: ['login'], tags: ['smoke', 'auth'], statuses: ['review'] })
  })

  it('drops a term with nothing after it, so it does not empty the list while it is typed', () => {
    const q = parseQuery('tag: # status: x')

    expect(q).toMatchObject({ words: ['x'], tags: [], statuses: [] })
    expect(hasLabelTerms(q)).toBe(false)
  })

  it('folds accents and case like every other search', () => {
    expect(parseQuery('Café').words).toEqual(['cafe'])
  })
})

describe('labelsMatch', () => {
  const item = { status: mark('in-development'), tags: ['Smoke', 'auth'] }

  it('needs every tag term, as a prefix', () => {
    expect(labelsMatch(item, parseQuery('tag:smo #au'))).toBe(true)
    expect(labelsMatch(item, parseQuery('tag:smoke tag:nightly'))).toBe(false)
  })

  it('takes any of the status terms, since an item has one', () => {
    expect(labelsMatch(item, parseQuery('status:review status:in-dev'))).toBe(true)
    expect(labelsMatch(item, parseQuery('status:review'))).toBe(false)
  })

  it('lets an item with no status or tags fail a term and pass a query of words only', () => {
    expect(labelsMatch({}, parseQuery('status:review'))).toBe(false)
    expect(labelsMatch({}, parseQuery('tag:x'))).toBe(false)
    expect(labelsMatch({}, parseQuery('login'))).toBe(true)
  })
})

describe('labelText', () => {
  it('is the status name and the tags', () => {
    expect(labelText({ status: mark('review'), tags: ['a', 'b'] })).toBe('review a b')
    expect(labelText({})).toBe('')
  })
})

describe('what a mark says', () => {
  it('reads a name as words', () => {
    expect(statusWords('in-development')).toBe('in development')
  })

  it('says where an inherited status comes from, and says so of a name the collection does not have', () => {
    expect(statusTitle(mark('review'))).toBe('Status: review')
    expect(statusTitle(mark('review', { inherited: true, from: 'requests/auth/legacy' }))).toBe('Status: review (from the folder auth/legacy)')
    expect(statusTitle(mark('reveiw', { unknown: true }))).toContain('not one of this collection')
  })

  it('draws a colour from its theme token, gray when it has none', () => {
    expect(statusColor('purple')).toBe('var(--st-purple)')
    expect(statusColor('')).toBe('var(--st-gray)')
  })
})

describe('cleaning what is typed', () => {
  it('makes a tag one word and drops empties and repeats, keeping the order given', () => {
    expect(cleanTag(' needs review ')).toBe('needs-review')
    expect(cleanTag('a,b')).toBe('a-b')
    expect(cleanTag(' - ')).toBe('')
    expect(cleanTags(['smoke', ' smoke', '', 'needs review', 'auth'])).toEqual(['smoke', 'needs-review', 'auth'])
  })

  it('makes a status name lower case with dashes for spaces', () => {
    expect(cleanStatusName('  Needs   Design ')).toBe('needs-design')
  })
})

describe('suggestTags', () => {
  const inUse = [{ tag: 'smoke', count: 5 }, { tag: 'slow', count: 3 }, { tag: 'auth', count: 2 }, { tag: 'nightly', count: 1 }]

  it('offers the tags in use that are not added yet, most used first', () => {
    expect(suggestTags(inUse, ['smoke'], '').map((s) => s.tag)).toEqual(['slow', 'auth', 'nightly'])
  })

  it('narrows to what was typed, wherever it is in the tag', () => {
    expect(suggestTags(inUse, [], 'l').map((s) => s.tag)).toEqual(['slow', 'nightly'])
    expect(suggestTags(inUse, [], 'zz')).toEqual([])
  })

  it('offers at most max', () => {
    expect(suggestTags(inUse, [], '', 2)).toHaveLength(2)
  })
})

// The colours a status may have are theme tokens (app.css --st-*), so each can be held to a contrast on every theme.
// The test runner leaves a ?raw CSS import empty, so the file is read from disk, as contrast.test.ts does.
// @ts-ignore
const { readFileSync } = (await import('node:fs')) as { readFileSync: (url: URL, enc: string) => string }
const sheet = readFileSync(new URL('../app.css', import.meta.url), 'utf8')

const COLORS = ['gray', 'blue', 'sky', 'teal', 'green', 'amber', 'red', 'pink', 'purple']
const hexes = (body: string) => Object.fromEntries([...body.matchAll(/--([a-z-]+):\s*(#[0-9a-fA-F]{6})\b/g)].map((m) => [m[1], m[2]]))
const mix = (a: string, b: string, pa: number) => '#' + [1, 3, 5].map((i) => Math.round(parseInt(a.slice(i, i + 2), 16) * pa + parseInt(b.slice(i, i + 2), 16) * (1 - pa)).toString(16).padStart(2, '0')).join('')

function setAfter(marker: string): Record<string, string> {
  const at = sheet.indexOf(marker)
  expect(at, marker).toBeGreaterThan(-1)
  return hexes(sheet.slice(at, sheet.indexOf('}', at)))
}
const dark = setAfter(':root { --st-gray')
const light = setAfter(":root[data-mode='light'] { --st-gray")

/** Each theme's pane, panel and frame colours, and whether it is a light one. */
function themes(): Record<string, { bg: string; panel: string; chrome: string; light: boolean }> {
  const root = sheet.slice(sheet.indexOf(':root {'), sheet.indexOf('}', sheet.indexOf(':root {')))
  const base = hexes(root)
  const out: Record<string, { bg: string; panel: string; chrome: string; light: boolean }> = {}
  for (const m of sheet.matchAll(/^((?:\[data-theme='[^']+'\](?:, )?)+) \{([^}]*)\}/gm)) {
    const v = { ...base, ...hexes(m[2]) }
    const isLight = /color-scheme:\s*light/.test(m[2])
    for (const n of m[1].matchAll(/data-theme='([^']+)'/g)) {
      const name = n[1]
      // The frame the themes that do not name one get (app.css, "The themes that name only --bg and --panel").
      const chrome =
        name === 'high-contrast' ? '#000000' : v.chrome && v.chrome !== base.chrome ? v.chrome : isLight ? (name === 'light' || name === 'flowi-light' ? '#f1f2f5' : mix(v.bg, v.text, 0.92)) : name === 'dark' || name === 'classic' || name === 'flowi-dark' ? '#0c0d10' : mix(v.bg, '#000000', 0.62)
      out[name] = { bg: v.bg, panel: v.panel, chrome, light: isLight }
    }
  }
  return out
}

describe('status colours', () => {
  it('has a token for each colour the library offers, in both sets', () => {
    expect(Object.keys(dark).filter((k) => k.startsWith('st-')).sort()).toEqual(COLORS.map((c) => 'st-' + c).sort())
    expect(Object.keys(light).filter((k) => k.startsWith('st-')).sort()).toEqual(COLORS.map((c) => 'st-' + c).sort())
  })

  it('gives the light set to the system light theme as well as to data-mode light', () => {
    const media = setAfter(':not([data-theme]) { --st-gray')

    expect(media).toEqual(light)
  })

  // Before: no status colour existed. A status is a colour and an icon, so the colour must read as the icon's ink.
  it('reach 4.5:1 on the pane, the panel and the frame of every theme', () => {
    const all = themes()
    const low: string[] = []

    for (const [name, t] of Object.entries(all)) {
      for (const color of COLORS) {
        const ink = (t.light ? light : dark)['st-' + color]
        for (const [where, bg] of Object.entries({ bg: t.bg, panel: t.panel, chrome: t.chrome })) {
          const c = contrastRatio(ink, bg)
          if (c < 4.5) low.push(`${name}: ${color} ${ink} on ${where} ${bg} is ${c.toFixed(2)}:1`)
        }
      }
    }

    expect(Object.keys(all).length).toBeGreaterThanOrEqual(8)
    expect(low).toEqual([])
  })
})

describe('status icons', () => {
  // The library names the icons a status may have (model.StatusIcons); the window draws each.
  const goSource = readFileSync(new URL('../../../../lib/model/status.go', import.meta.url), 'utf8')
  const listed = (fn: string) => [...(new RegExp(`func ${fn}\\(\\) \\[\\]string \\{([\\s\\S]*?)\\n\\}`).exec(goSource)?.[1].matchAll(/"([a-z0-9-]+)"/g) ?? [])].map((m) => m[1])

  it('has a drawing for every icon the library offers', () => {
    const names = listed('StatusIcons')

    expect(names.length).toBeGreaterThanOrEqual(10)
    expect(names.filter((n) => !(n in ICONS))).toEqual([])
  })

  it('has a token for every colour the library offers', () => {
    expect(listed('StatusColors')).toEqual(COLORS)
  })

  it('draws every default status with an icon and a colour of its own', () => {
    const defaults = [...goSource.matchAll(/\{Name: "([a-z-]+)", Color: "([a-z]+)", Icon: "([a-z]+)"\}/g)].map((m) => ({ name: m[1], color: m[2], icon: m[3] }))

    expect(defaults.map((d) => d.name)).toEqual(['planning', 'in-development', 'review', 'testing', 'production'])
    expect(new Set(defaults.map((d) => d.color)).size).toBe(5)
    expect(new Set(defaults.map((d) => d.icon)).size).toBe(5)
    for (const d of defaults) {
      expect(COLORS, d.name).toContain(d.color)
      expect(d.icon in ICONS, d.name).toBe(true)
    }
  })
})

describe('labelChoices', () => {
  it('Given a tree When read Then it lists the statuses met once and the tags most used first', () => {
    const tree = [{ status: mark('review'), tags: ['smoke'], children: [{ status: mark('review'), tags: ['smoke', 'auth'] }, { status: mark('testing'), tags: ['auth', 'smoke'] }] }]
    const got = labelChoices(tree)
    expect(got.statuses.map((s) => s.name)).toEqual(['review', 'testing'])
    expect(got.tags).toEqual([{ tag: 'smoke', count: 3 }, { tag: 'auth', count: 2 }])
  })
})

describe('toggleTerm', () => {
  it('Given a term not in the query When toggled Then it is added, and a second toggle takes it off', () => {
    expect(toggleTerm('login', 'tag:smoke')).toBe('login tag:smoke')
    expect(toggleTerm('login tag:smoke', 'tag:smoke')).toBe('login')
    expect(toggleTerm('', 'status:review')).toBe('status:review')
  })
})
