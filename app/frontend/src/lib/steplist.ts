import { statusMark } from './statusmark'
import type { TimelineBar, Unit } from './types'

/** One line of the step outline: a step of the flow, where it sits, and how its last run went. */
export interface StepRow {
  id: string
  type: string
  /** How many containers it sits inside: the indent. */
  depth: number
  status: string
  /** The shape that goes with the status; "" before a run. */
  mark: string
  /** The card's result line ("passed · 120ms"); "" before a run. */
  result: string
  /** The step's bar as shares of the run, or null when it did not run. */
  bar: { left: number; width: number } | null
}

/**
 * The steps of the flow as a list, in the canvas's order: what a long flow needs when its canvas is thousands of
 * pixels wide. Before: the only way to find step 19 was to scroll the canvas to it.
 */
export function stepRows(units: Unit[], statuses: Record<string, string>, results: Record<string, string>, bars: TimelineBar[]): StepRow[] {
  const total = bars.reduce((end, b) => Math.max(end, b.endMs), 0)
  const barOf = new Map(bars.map((b) => [b.step, b]))
  const parentOf = new Map(units.map((u) => [u.id, u.parent]))
  const depthOf = (unit: Unit): number => {
    let depth = 0
    const seen = new Set<string>()
    for (let p = unit.parent; p && !seen.has(p); p = parentOf.get(p)) {
      seen.add(p)
      depth++
    }
    return depth
  }
  const rows: StepRow[] = []
  const listed = new Set<string>()
  for (const u of units) {
    const depth = depthOf(u)
    u.nodes.forEach((id, i) => {
      if (listed.has(id)) return
      listed.add(id)
      const b = barOf.get(id)
      const status = statuses[id] ?? ''
      rows.push({
        id,
        type: u.types[i] ?? '',
        depth,
        status,
        mark: status ? statusMark(status) : '',
        result: results[id] ?? '',
        bar: b && total > 0 ? { left: (b.startMs / total) * 100, width: Math.max(0.5, ((b.endMs - b.startMs) / total) * 100) } : null,
      })
    })
  }
  return rows
}

/** Keeps the rows whose id or type contains the text, ignoring case; every row for an empty text. */
export function filterRows(rows: StepRow[], text: string): StepRow[] {
  const q = text.trim().toLowerCase()
  return q ? rows.filter((r) => r.id.toLowerCase().includes(q) || r.type.toLowerCase().includes(q)) : rows
}
