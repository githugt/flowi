import { describe, expect, it } from 'vitest'

// A second ratchet beside designsystem.test.ts, for the rules UI7-D6 lists: a component may not add a window
// pointer-down closer (Popover and Menu do that), a typed "×" or a caret character (IconButton and Icon draw them), a
// colour written as rgb(), a radius or font size in pixels (tokens), or a raw select or checkbox. The counts below are
// what exists today, per file; a count may go down (lower the number) but not up.
const components = import.meta.glob('../*.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

const RULES = ['windowPointerdown', 'typedX', 'caret', 'rgba', 'pxRadius', 'pxFontSize', 'rawSelect', 'rawCheckbox'] as const
type Row = readonly number[]

// file: one count per rule, in the order of RULES
const BASELINE: Record<string, Row> = {
  'App.svelte': [0, 0, 0, 0, 1, 1, 0, 0],
  'Canvas.svelte': [0, 0, 0, 0, 0, 4, 0, 0],
  'Checkbox.svelte': [0, 0, 0, 0, 0, 0, 0, 1],
  'MethodPicker.svelte': [1, 0, 0, 0, 0, 0, 0, 0],
  'Rail.svelte': [1, 0, 0, 0, 0, 0, 0, 0],
  'RequestEditor.svelte': [0, 0, 0, 0, 1, 0, 0, 0],
  'RunsTab.svelte': [0, 0, 0, 0, 0, 1, 0, 0],
  'SchedulesScreen.svelte': [0, 0, 0, 0, 2, 0, 0, 0],
  'Select.svelte': [0, 0, 0, 0, 0, 0, 1, 0],
  'SequenceDiagram.svelte': [0, 0, 0, 0, 0, 3, 0, 0],
}

const style = (text: string) => /<style[^>]*>([\s\S]*?)<\/style>/.exec(text)?.[1] ?? ''
const count = (text: string, pattern: RegExp) => (text.match(pattern) ?? []).length

function counts(text: string): number[] {
  const css = style(text)
  return [
    count(text, /<svelte:window[^>]*onpointerdown/g),
    count(text, /(?:>|\s)×\s*</g),
    count(text, /[▾▸]/g),
    count(css, /rgba?\(/g),
    count(css, /border-radius:\s*\d+px/g),
    count(css, /font-size:\s*\d+(?:\.\d+)?px/g),
    count(text, /<select\b/g),
    count(text, /type="checkbox"/g),
  ]
}

describe('design rules ratchet', () => {
  it('lets no component add to a count', () => {
    const over: string[] = []
    for (const [path, text] of Object.entries(components)) {
      const file = path.replace('../', '')
      const have = counts(text)
      const base = BASELINE[file] ?? []
      RULES.forEach((rule, i) => {
        if (have[i] > (base[i] ?? 0)) over.push(`${file}: ${have[i]} ${rule}, the baseline is ${base[i] ?? 0}`)
      })
    }

    expect(Object.keys(components).length).toBeGreaterThan(20)
    expect(over).toEqual([])
  })

  it('has no baseline entry for a file that is gone', () => {
    const files = new Set(Object.keys(components).map((p) => p.replace('../', '')))

    expect(Object.keys(BASELINE).filter((f) => !files.has(f))).toEqual([])
  })

  it('names every @keyframes once, so a second copy of one is a reuse, not a rewrite', () => {
    // Before: the toast, the dialog and the menu each wrote their own rise-and-fade.
    const seen = new Map<string, string[]>()
    for (const [path, text] of Object.entries(components)) {
      for (const m of text.matchAll(/@keyframes\s+([\w-]+)/g)) seen.set(m[1], [...(seen.get(m[1]) ?? []), path.replace('../', '')])
    }

    expect([...seen].filter(([, files]) => files.length > 1)).toEqual([])
  })
})
