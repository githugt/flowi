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

/** The row of the environment called name, wherever the tree puts it. */
export function environmentNode(roots: TreeNode[] | undefined, name: string): TreeNode | undefined {
  const find = (nodes: TreeNode[]): TreeNode | undefined => {
    for (const n of nodes) {
      if (n.kind === 'environment' && n.label === name) return n
      const inner = find(n.children ?? [])
      if (inner) return inner
    }
  }
  return find(roots ?? [])
}

/** The row with this id, wherever the tree puts it. */
export function nodeById(nodes: TreeNode[] | undefined, id: string): TreeNode | undefined {
  for (const n of nodes ?? []) {
    if (n.id === id) return n
    const inner = nodeById(n.children, id)
    if (inner) return inner
  }
}
