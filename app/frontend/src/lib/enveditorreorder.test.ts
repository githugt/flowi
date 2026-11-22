import { describe, expect, it } from 'vitest'
import { moved } from './reorder'

// Before: the variables of an environment stayed in the order they were added, and the only way to change it was to
// delete rows and type them again.
describe('reordering the variables of an environment', () => {
  it('should move a row to a new place and keep the others in order', () => {
    expect(moved(['a', 'b', 'c', 'd'], 0, 2)).toEqual(['b', 'c', 'a', 'd'])
    expect(moved(['a', 'b', 'c', 'd'], 3, 1)).toEqual(['a', 'd', 'b', 'c'])
  })

  it('should leave the list alone for a move that goes nowhere or off the ends', () => {
    const list = ['a', 'b']

    expect(moved(list, 0, 0)).toBe(list)
    expect(moved(list, 0, -1)).toBe(list)
    expect(moved(list, 1, 2)).toBe(list)
  })

  it('should be wired into the editor: a grip per row, the order written on drop and on a key, off while filtering', async () => {
    const src = (await import('../EnvEditor.svelte?raw')).default as string

    expect(src).toContain('icon="grip"')
    expect(src).toContain('rows = moved(rows')
    expect(src).toMatch(/function drop\(\) \{[^}]*push\(\)/)
    expect(src).toMatch(/function gripKeys[\s\S]*push\(\)/)
    expect(src).toContain('disabled={!!filter}')
  })
})
