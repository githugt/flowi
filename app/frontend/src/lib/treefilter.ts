import type { TreeNode } from './types'
import { fold } from './fold'
import { hasLabelTerms, labelsMatch, labelText, parseQuery, type LabelQuery } from './devstatus'

// Rows that open something when clicked; every other kind is a group that holds rows.
const LEAF_KINDS = ['request', 'environment', 'flow', 'missing', 'broken']

const isLeaf = (n: TreeNode): boolean => LEAF_KINDS.includes(n.kind)

// The words of a query that mark a label: `tag:x`, `#x` and `status:y` are terms about a row's tags and status
// (lib/devstatus), not text to find in its name, so they are not highlighted in it. Lower case only, not folded, so the
// stretches found are the same length in the label.
const wordsOf = (query: string): string[] =>
  query
    .toLowerCase()
    .split(/\s+/)
    .filter((w) => w && !/^(#|tag:|status:)/.test(w))

function matchesAll(n: TreeNode, q: LabelQuery): boolean {
  // A row is found by its name and verb as before, and now by its status name and its tags too.
  const text = fold(`${n.label} ${n.method ?? ''} ${labelText(n)}`)
  return q.words.every((w) => text.includes(w)) && labelsMatch(n, q)
}

function keep(n: TreeNode, q: LabelQuery): TreeNode | undefined {
  // A group whose own name matches shows whole: the user named the place, so everything in it is a hit. Not when the
  // query asks for a tag or a status, which are about the rows and not the place.
  if (!hasLabelTerms(q) && (isLeaf(n) ? matchesAll(n, q) : q.words.every((w) => fold(n.label).includes(w)))) return n
  if (isLeaf(n)) return hasLabelTerms(q) && matchesAll(n, q) ? n : undefined
  const kids = (n.children ?? []).flatMap((c) => keep(c, q) ?? [])
  return kids.length ? { ...n, children: kids } : undefined
}

/**
 * The part of the tree that answers a "Jump to…" query: rows whose label, method, status or tags contain every word,
 * with the groups that lead to them; `tag:smoke` and `status:review` narrow it to rows with that tag or status. The
 * tree itself comes back for a blank query. Nothing is changed in place.
 */
export function filterTree(roots: TreeNode[], query: string): TreeNode[] {
  const q = parseQuery(query)
  if (!q.words.length && !hasLabelTerms(q)) return roots
  return roots.flatMap((r) => keep(r, q) ?? [])
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
