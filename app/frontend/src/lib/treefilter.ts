import type { TreeNode } from './types'

// Rows that open something when clicked; every other kind is a group that holds rows.
const LEAF_KINDS = ['request', 'environment', 'flow', 'missing']

const isLeaf = (n: TreeNode): boolean => LEAF_KINDS.includes(n.kind)

const wordsOf = (query: string): string[] => query.toLowerCase().split(/\s+/).filter(Boolean)

function matchesAll(n: TreeNode, words: string[]): boolean {
  const text = `${n.label} ${n.method ?? ''}`.toLowerCase()
  return words.every((w) => text.includes(w))
}

function keep(n: TreeNode, words: string[]): TreeNode | undefined {
  // A group whose own name matches shows whole: the user named the place, so everything in it is a hit.
  if (matchesAll(n, words)) return n
  const kids = (n.children ?? []).flatMap((c) => keep(c, words) ?? [])
  return kids.length ? { ...n, children: kids } : undefined
}

/**
 * The part of the tree that answers a "Jump to…" query: rows whose label or method contains every word, with the
 * groups that lead to them. The tree itself comes back for a blank query. Nothing is changed in place.
 */
export function filterTree(roots: TreeNode[], query: string): TreeNode[] {
  const words = wordsOf(query)
  if (!words.length) return roots
  return roots.flatMap((r) => keep(r, words) ?? [])
}

/** The first row, top to bottom, that opens something rather than holding rows. */
export function firstLeaf(nodes: TreeNode[]): TreeNode | undefined {
  for (const n of nodes) {
    if (isLeaf(n)) return n
    const found = firstLeaf(n.children ?? [])
    if (found) return found
  }
  return undefined
}

export interface LabelPart {
  text: string
  hit: boolean
}

/** A label cut into the stretches the query's words match and the stretches between, for showing the hits. */
export function highlightParts(label: string, query: string): LabelPart[] {
  const lower = label.toLowerCase()
  const spans = wordsOf(query).flatMap((w) => {
    const at = lower.indexOf(w)
    return at < 0 ? [] : [[at, at + w.length] as [number, number]]
  })
  spans.sort((a, b) => a[0] - b[0])
  const merged: [number, number][] = []
  for (const [from, to] of spans) {
    const last = merged[merged.length - 1]
    if (last && from <= last[1]) last[1] = Math.max(last[1], to)
    else merged.push([from, to])
  }
  const parts: LabelPart[] = []
  let at = 0
  for (const [from, to] of merged) {
    if (from > at) parts.push({ text: label.slice(at, from), hit: false })
    parts.push({ text: label.slice(from, to), hit: true })
    at = to
  }
  if (at < label.length || !parts.length) parts.push({ text: label.slice(at), hit: false })
  return parts
}
