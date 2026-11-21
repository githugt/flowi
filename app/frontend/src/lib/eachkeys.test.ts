import { describe, expect, it } from 'vitest'
import { eachBlocks } from './eachkeys'

const files = import.meta.glob('../*.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

describe('eachBlocks', () => {
  it('tells a keyed block from an unkeyed one and sees an edited row', () => {
    const text = ['{#each rows as r (r.id)}', '<p>{r.name}</p>', '{/each}', '{#each rows as r, i}', '<input bind:value={r.name} />', '{/each}'].join('\n')

    const blocks = eachBlocks(text)

    expect(blocks.map((b) => [b.line, b.keyed, b.editable])).toEqual([
      [1, true, false],
      [4, false, true],
    ])
  })
})

describe('keys on lists', () => {
  const all = Object.entries(files).flatMap(([file, text]) => eachBlocks(text).map((b) => ({ file: file.slice(3), ...b })))

  // Before: 197 of 258 lists had no key, and nothing counted them. Both numbers may only go down; to lower one,
  // key the rows by a stable id (not the index) and lower the number here.
  it('does not gain unkeyed lists', () => {
    const unkeyed = all.filter((b) => !b.keyed).length
    expect(all.length).toBeGreaterThan(200) // the glob still finds them
    expect(unkeyed).toBeLessThanOrEqual(LIMIT_UNKEYED)
  })
  it('does not gain unkeyed lists that are edited in place', () => {
    const editable = all.filter((b) => !b.keyed && b.editable).map((b) => `${b.file}:${b.line}`)
    expect(editable.length, editable.join(', ')).toBeLessThanOrEqual(LIMIT_EDITABLE)
  })
})

const LIMIT_UNKEYED = 197
const LIMIT_EDITABLE = 11
