import { describe, expect, it } from 'vitest'
import menus from './menus.ts?raw'

describe('the New menu', () => {
  const first = menus.slice(menus.indexOf('export function newMenu'), menus.indexOf('export function newInCollectionMenu'))
  const second = menus.slice(menus.indexOf('export function newInCollectionMenu'), menus.indexOf('const protocols'))
  const labels = (src: string) => [...src.matchAll(/label: '([^']+)'/g)].map((m) => m[1]).filter((l) => l)

  it('has eleven items, with the kinds made in a collection behind one entry', () => {
    // Before: 17 items in one list, six of them the same kinds again "in a collection…".
    expect(labels(first)).toHaveLength(11)
    expect(labels(first)).toContain('New in a collection…')
    expect(first).toContain('newInCollectionMenu()')
  })
  it('asks for a name before it makes a GraphQL or gRPC request in Scratch', () => {
    // Before: the two entries made the request at once, so the name could not be chosen.
    expect(first).toContain("newScratchNamed('graphql'")
    expect(first).toContain("newScratchNamed('grpc'")
    expect(first).not.toContain("NewAt('graphql'")
    expect(first).not.toContain("NewAt('grpc'")
  })
  it('keeps every kind in the second list', () => {
    expect(labels(second)).toEqual(['Request…', 'Message request…', 'GraphQL request…', 'gRPC request…', 'Database query…', 'Data file query…', 'Flow…'])
  })
})
