import { describe, expect, it } from 'vitest'
import { collectionRoot, firstOfKind } from './treewalk'
import type { TreeNode } from './types'

const tree: TreeNode[] = [
  { id: 'c:C1', label: 'Payments', kind: 'collection', children: [{ id: 'g', label: 'requests', kind: 'requests', children: [{ id: 'r1', label: 'GET /pets', kind: 'request' }] }] },
  { id: 'c:C2', label: 'Other', kind: 'collection' },
]

describe('collectionRoot', () => {
  // Before: the Unmount note looked for a root with the bare id, which the tree never has ("c:" comes first).
  it('finds a collection by its bare id or by the tree id', () => {
    expect(collectionRoot(tree, 'C1')?.label).toBe('Payments')
    expect(collectionRoot(tree, 'c:C2')?.label).toBe('Other')
    expect(collectionRoot(tree, 'nope')).toBeUndefined()
    expect(collectionRoot(undefined, 'C1')).toBeUndefined()
  })
})

describe('firstOfKind', () => {
  it('finds the first request under a collection', () => {
    expect(firstOfKind('request', tree[0].children ?? [])?.id).toBe('r1')
    expect(firstOfKind('flow', tree)).toBeUndefined()
  })
})
