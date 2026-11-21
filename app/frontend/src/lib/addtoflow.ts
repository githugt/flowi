/** An open flow tab a request can be added to. */
export interface FlowTarget {
  node: string
  label: string
}

const REQUEST = /^r:([^:]+):(.+)$/

/** The request a tree row stands for: its collection and id; null for any other row. */
export function requestOfRow(row: string): { collection: string; id: string } | null {
  const m = REQUEST.exec(row)
  return m ? { collection: m[1], id: m[2] } : null
}

/**
 * The open flows that can take the request of a tree row as a step: flows of the same collection. A flow takes
 * requests of its own collection (and its dependencies, which a tree row does not say), so others are left out.
 * Before: a request could be put into a flow only through the flat select of an empty request step.
 */
export function addToFlowTargets(row: string, tabs: { node: string; label: string; kind: string }[]): FlowTarget[] {
  const r = requestOfRow(row)
  if (!r) return []
  return tabs.filter((t) => t.kind === 'flow' && t.node.startsWith(`f:${r.collection}:`)).map((t) => ({ node: t.node, label: t.label }))
}
