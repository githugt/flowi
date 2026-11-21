import { describe, expect, it } from 'vitest'
import { menuItemForKey, rowFor, typeAhead, typedChar, TYPE_AHEAD_MS } from './typeahead'

const labels = ['auth', 'users', 'usage', 'orders']
const key = (k: string, o = {}) => ({ key: k, ctrlKey: false, altKey: false, metaKey: false, ...o })

describe('rowFor', () => {
  it('goes to the first and the last row on Home and End', () => {
    // Before: only the arrow keys moved through the tree.
    expect(rowFor(labels, 2, 'Home', '')).toBe(0)
    expect(rowFor(labels, 0, 'End', '')).toBe(3)
  })
  it('goes to the next row that starts with the typed letter, wrapping round', () => {
    expect(rowFor(labels, 0, 'u', 'u')).toBe(1)
    expect(rowFor(labels, 1, 'u', 'u')).toBe(2)
    expect(rowFor(labels, 2, 'u', 'u')).toBe(1)
  })
  it('stays on the current row while more letters still match it', () => {
    expect(rowFor(labels, 1, 's', 'us')).toBe(1)
    expect(rowFor(labels, 1, 'a', 'usa')).toBe(2)
  })
  it('stays put when nothing matches or nothing was typed', () => {
    expect(rowFor(labels, 1, 'z', 'z')).toBe(-1)
    expect(rowFor(labels, 1, 'Tab', '')).toBe(-1)
    expect(rowFor([], 0, 'Home', '')).toBe(-1)
  })
})

describe('typeAhead', () => {
  it('joins letters typed close together and starts over after a pause', () => {
    const a = typeAhead({ buffer: '', at: 0 }, 'u', 100)
    const b = typeAhead(a, 's', 100 + TYPE_AHEAD_MS)
    expect(b.buffer).toBe('us')
    expect(typeAhead(b, 'o', b.at + TYPE_AHEAD_MS + 1).buffer).toBe('o')
  })
})

describe('typedChar', () => {
  it('takes one printable character and ignores shortcuts, space and named keys', () => {
    expect(typedChar(key('U'))).toBe('u')
    expect(typedChar(key('u', { ctrlKey: true }))).toBe('')
    expect(typedChar(key(' '))).toBe('')
    expect(typedChar(key('Enter'))).toBe('')
  })
})

describe('the sidebar', () => {
  it('filters 80 ms after the last keystroke and marks the leaf rows content-visibility: auto', async () => {
    // Before: every letter typed in Jump re-filtered the whole tree and every row of a long list was laid out.
    // @ts-ignore node module in a vitest-only test
    const { readFileSync } = await import('node:fs')
    const read = (f: string) => readFileSync(new URL('../' + f, import.meta.url), 'utf8') as string
    const { JUMP_DELAY_MS } = await import('./typeahead')
    expect(JUMP_DELAY_MS).toBe(80)
    expect(read('Sidebar.svelte')).toContain('debounced((q: string) => (filterQuery = q), JUMP_DELAY_MS)')
    expect(read('Sidebar.svelte')).toContain('settle.flush()')
    expect(read('TreeItem.svelte')).toContain('content-visibility: auto')
  })
})

describe('menuItemForKey', () => {
  const items = [{ label: 'Open' }, { label: 'Rename…' }, { label: 'Delete 3…' }]
  it('maps F2 to Rename and Delete to Delete', () => {
    // Before: renaming or deleting a row needed the right-click menu.
    expect(menuItemForKey(items, 'F2')?.label).toBe('Rename…')
    expect(menuItemForKey(items, 'Delete')?.label).toBe('Delete 3…')
  })
  it('is null for other keys and for a menu without the item', () => {
    expect(menuItemForKey(items, 'F3')).toBeNull()
    expect(menuItemForKey([{ label: 'Open' }], 'F2')).toBeNull()
  })
})
