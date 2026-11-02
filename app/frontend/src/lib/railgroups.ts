import type { TreeNode } from './types'

export type RailGroup = 'collections' | 'environments' | 'flows'

/**
 * The part of the tree a rail icon shows: the collections whole, the environments alone, or only the flows of each
 * collection (a collection without any is left out).
 */
export function rootsForGroup(roots: TreeNode[], group: RailGroup | ''): TreeNode[] {
  if (!group) return roots
  if (group === 'environments') return roots.filter((r) => r.kind === 'environments')
  const collections = roots.filter((r) => r.kind !== 'environments')
  if (group === 'collections') return collections
  return collections
    .map((c) => ({ ...c, children: (c.children ?? []).filter((k) => k.kind === 'flows') }))
    .filter((c) => c.children.length > 0)
}
