// Finding things in the workspace tree the sidebar shows.
import type { TreeNode } from './types'

/** The root row of a collection, by the collection's id ("c:" is how the tree writes it). */
export function collectionRoot(roots: TreeNode[] | undefined, id: string): TreeNode | undefined {
  return roots?.find((r) => r.kind === 'collection' && (r.id === id || r.id === 'c:' + id))
}

/** The first node of a kind at or under the nodes, in tree order. */
export function firstOfKind(kind: string, nodes: TreeNode[]): TreeNode | undefined {
  for (const n of nodes) {
    if (n.kind === kind) return n
    const inner = firstOfKind(kind, n.children ?? [])
    if (inner) return inner
  }
}
