import { describe, expect, it } from 'vitest'
import { dependentsOf, subtreeIds, tabsIn, unmountNote } from './unmount'

const root = { id: 'c1', label: 'Payments', kind: 'collection', children: [{ id: 'f1', label: 'f', kind: 'folder', children: [{ id: 'r1', label: 'r', kind: 'request' }] }] }

describe('unmount note', () => {
  it('counts the open tabs inside the collection only', () => {
    expect([...subtreeIds(root)].sort()).toEqual(['c1', 'f1', 'r1'])
    expect(tabsIn(root, [{ node: 'r1' }, { node: 'other' }, { node: 'c1' }])).toBe(2)
    expect(tabsIn(undefined, [{ node: 'r1' }])).toBe(0)
  })
  it('finds the collections that depend on it', () => {
    const all = [{ name: 'Payments', deps: [] }, { name: 'Billing', deps: [{ name: 'Payments' }] }, { name: 'Misc', deps: [{ name: 'X' }] }]

    expect(dependentsOf('Payments', all)).toEqual(['Billing'])
  })
  it('says what stays, and what closes or breaks', () => {
    expect(unmountNote('Payments', 0, [])).toBe('Your files are not touched; the collection can be mounted again.')
    expect(unmountNote('Payments', 3, ['Billing'])).toContain('3 tabs open in it will be closed.')
    expect(unmountNote('Payments', 1, ['Billing', 'Ops'])).toContain('Billing, Ops depend on Payments')
  })
})
