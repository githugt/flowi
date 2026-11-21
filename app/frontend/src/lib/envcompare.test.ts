import { describe, expect, it } from 'vitest'
import { cellLabel, cellText, summary, visibleRows } from './envcompare'
import type { EnvCompareView } from './types'

type Row = EnvCompareView['rows'][number]
const set = (value: string) => ({ state: 'set' as const, value, kind: '', source: '' })
const row = (name: string, over: Partial<Row> = {}): Row => ({ name, cells: [set('a'), set('a')], differs: false, gaps: 0, ...over })

describe('visibleRows', () => {
  it('should keep every row, or only the ones that differ or have a gap', () => {
    // Before: finding why staging behaved unlike local meant opening each environment and comparing by eye.
    const rows = [row('same'), row('differs', { differs: true }), row('gap', { gaps: 1 })]

    expect(visibleRows(rows, false).map((r) => r.name)).toEqual(['same', 'differs', 'gap'])
    expect(visibleRows(rows, true).map((r) => r.name)).toEqual(['differs', 'gap'])
  })
})

describe('cellText', () => {
  it('should never show a secret, only whether it has a value', () => {
    expect(cellText({ state: 'secret', value: '', kind: '', source: '' })).toBe('● set')
    expect(cellText({ state: 'secret-unset', value: '', kind: '', source: '' })).toBe('○ no value')
  })

  it('should show a dash for a missing variable, the value for a set one and (empty) for an empty text', () => {
    expect(cellText({ state: 'missing', value: '', kind: '', source: '' })).toBe('—')
    expect(cellText(set('https://x'))).toBe('https://x')
    expect(cellText(set(''))).toBe('(empty)')
  })
})

describe('cellLabel', () => {
  it('should say what the cell is and, for an inherited one, where it comes from', () => {
    expect(cellLabel('staging', 'region', { state: 'inherited', value: 'eu', kind: '', source: 'base' })).toBe('region in staging: eu, from base')
    expect(cellLabel('prod', 'token', { state: 'secret-unset', value: '', kind: '', source: '' })).toBe('token in prod: a secret without a value')
    expect(cellLabel('local', 'debug', { state: 'missing', value: '', kind: '', source: '' })).toBe('debug in local: not defined')
  })

  it('should say when a value is read as something other than text, since the same characters differ by type', () => {
    expect(cellLabel('local', 'retries', { state: 'set', value: '3', kind: 'number', source: '' })).toBe('retries in local: 3 (read as number)')
  })
})

describe('summary', () => {
  it('should count what differs and what has gaps', () => {
    expect(summary([])).toBe('No variables.')
    expect(summary([row('a'), row('b')])).toBe('2 variables, the same everywhere.')
    expect(summary([row('a', { differs: true }), row('b', { gaps: 2 }), row('c')])).toBe('3 variables: 1 differs, 1 has a gap.')
    expect(summary([row('a', { gaps: 1 }), row('b', { gaps: 1 })])).toBe('2 variables: 2 have gaps.')
  })
})

describe('the Compare environments screen', () => {
  it('should be opened from the menus and the palette and drawn by the app', async () => {
    const { screens } = await import('./screens')
    const { mainItems } = await import('./mainmenu')
    const app = (await import('../App.svelte?raw')).default as string
    const screen = (await import('../EnvCompareScreen.svelte?raw')).default as string

    expect(screens.some((s) => s.id === 'screen:envcompare' && s.menu === 'envcompare' && !s.rail)).toBe(true)
    expect(mainItems.some((i) => i.action === 'envcompare')).toBe(true)
    expect(app).toContain("tab.node === 'screen:envcompare'")
    expect(screen).toContain('EnvironmentService.Compare()')
  })
})
