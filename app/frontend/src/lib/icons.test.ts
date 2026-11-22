import { describe, expect, it } from 'vitest'
import { FALLBACK_ICON, ICONS, iconPath } from './icons'
import { rowIcon, typeIcon } from './card'

const hasIcon = (name: string) => name in ICONS

const sources = import.meta.glob(['../*.svelte', './*.ts', '!./*.test.ts'], { query: '?raw', import: 'default', eager: true }) as Record<string, string>

describe('icons', () => {
  it('draws a dot for a name that is not an icon, not nothing', () => {
    // Before: a missing name gave `d={undefined}`: an empty svg and no hint that the name was wrong.
    expect(iconPath('no-such-icon')).toBe(FALLBACK_ICON)
    expect(iconPath('close')).toBe(ICONS.close)
    expect(hasIcon('no-such-icon')).toBe(false)
  })

  it('has the glyphs that steps and controls used to share', () => {
    for (const name of ['stop', 'close', 'more', 'chevr', 'globe', 'hook', 'loop', 'repeat', 'note', 'log', 'flow', 'info', 'undo', 'redo']) {
      expect(hasIcon(name), name).toBe(true)
    }
  })

  it('gives no two glyphs the same drawing', () => {
    // Before: request, fetch and webhook shared one icon, and log and note another.
    const seen = new Map<string, string>()
    const twins: string[] = []
    for (const [name, path] of Object.entries(ICONS)) {
      const other = seen.get(path)
      if (other && !(name === 'assert' && other === 'checks') && !(name === 'checks' && other === 'assert') && !(name === 'binding' && other === 'hooks')) twins.push(`${other} = ${name}`)
      seen.set(path, name)
    }

    expect(twins).toEqual([])
  })

  it('only names icons that exist, in every step type and summary row', () => {
    const types = ['request', 'subflow', 'fetch', 'webhook', 'set', 'assert', 'condition', 'batch', 'run_till', 'parallel', 'delay', 'log', 'save', 'digest', 'stop', 'loop_control', 'note']
    const rows = ['inputs', 'send', 'receive', 'connection', 'hooks', 'outcomes', 'settings', 'values', 'checks', 'steps', 'message']

    expect(types.filter((t) => !hasIcon(typeIcon(t)))).toEqual([])
    expect(rows.filter((r) => !hasIcon(rowIcon(r)))).toEqual([])
    expect(new Set(['request', 'fetch', 'webhook'].map(typeIcon)).size).toBe(3)
    expect(typeIcon('log')).not.toBe(typeIcon('note'))
  })

  it('has every icon the screens name in their source', () => {
    const missing: string[] = []
    for (const [file, text] of Object.entries(sources)) {
      for (const m of text.matchAll(/<Icon name="([a-z0-9-]+)"|<IconButton icon="([a-z0-9-]+)"|\bicon: '([a-z0-9-]+)'/g)) {
        const name = m[1] ?? m[2] ?? m[3]
        if (!hasIcon(name)) missing.push(`${file}: ${name}`)
      }
    }

    expect(missing).toEqual([])
  })
})
