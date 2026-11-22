// The tooltip of a tree row: the whole name, which the row cuts with an ellipsis, and what kind of thing it is.
// Before: a long name was cut and nothing showed the rest.
import { statusTitle } from './devstatus'
import type { StatusMark } from './types'

/** "POST Create an order (request)"; "Checkout (flow)"; the bare label for anything else. */
export function rowTitle(node: { kind: string; label: string; method?: string; detail?: string; status?: StatusMark; tags?: string[] }): string {
  const base = baseTitle(node)
  const labels = [node.status ? statusTitle(node.status) : '', node.tags?.length ? `Tags: ${node.tags.join(', ')}` : ''].filter(Boolean)
  return labels.length ? `${base}\n${labels.join('\n')}` : base
}

function baseTitle(node: { kind: string; label: string; method?: string; detail?: string }): string {
  // A file that could not be read stays in the tree, with the reason on hover (before: it vanished as if deleted).
  if (node.kind === 'broken') return `${node.label}: cannot load, ${node.detail ?? 'the file is not valid'}`
  if (node.kind === 'request') return node.method ? `${node.method} ${node.label}` : node.label
  if (node.kind === 'flow') return `${node.label} (flow)`
  return node.label
}
