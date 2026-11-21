// Which steps a wire being dragged may end on. Go answers once when the drag starts (FlowService.WireTargets); the
// canvas dims the units that would be refused and says why under the pointer.
// Before: any drop was accepted, and a link the layout could not draw was saved and never shown.

export interface WireTarget {
  id: string
  ok: boolean
  reason: string
}

/** The kind of wire Go is asked about: an arrow (a port dragged to a step) or a value. */
export type WireKind = 'wait' | 'data'

/** The reason of each unit its first step cannot take, by unit id; the unit the wire starts from is never in it. */
export function blockedUnits(units: { id: string; nodes: string[] }[], targets: WireTarget[], origin: string): Record<string, string> {
  const byNode = new Map(targets.map((t) => [t.id, t]))
  const out: Record<string, string> = {}
  for (const u of units) {
    if (u.id === origin) continue
    const t = byNode.get(u.nodes[0])
    if (t && !t.ok) out[u.id] = t.reason || 'This step cannot take the link'
  }
  return out
}

/** The reason in a few words for the label by the pointer: the text before the explanation that follows a colon. */
export function hintOf(reason: string): string {
  return reason.split(':')[0].trim()
}
