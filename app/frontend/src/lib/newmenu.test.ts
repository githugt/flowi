import { describe, expect, it } from 'vitest'
import menus from './menus.ts?raw'

describe('the New menu', () => {
  const first = menus.slice(menus.indexOf('export function newMenu'), menus.indexOf('export function newInCollectionMenu'))
  const second = menus.slice(menus.indexOf('export function newInCollectionMenu'), menus.indexOf('const protocols'))
  const labels = (src: string) => [...src.matchAll(/label: '([^']+)'/g)].map((m) => m[1]).filter((l) => l)

  it('has ten items, with the kinds made in a collection behind one entry', () => {
    // Before: 17 items in one list, six of them the same kinds again "in a collection…".
    expect(labels(first)).toHaveLength(10)
    expect(labels(first)).toContain('New in a collection…')
    expect(first).toContain('newInCollectionMenu()')
  })
  it('keeps every kind in the second list', () => {
    expect(labels(second)).toEqual(['Request…', 'Message request…', 'gRPC request…', 'Database query…', 'Data file query…', 'Flow…'])
  })
})
