// What the step popup shows and how the keys move through it. Plain functions so the choices are tested
// without a window.
import type { StepChoice } from './types'

export interface StepGroup {
  name: string
  items: StepChoice[]
}

/** The steps whose name or description contains every word of query, ignoring case; all of them for a blank one. */
export function filterSteps(steps: StepChoice[], query: string): StepChoice[] {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean)
  return steps.filter((s) => {
    const text = `${s.name} ${s.description}`.toLowerCase()
    return words.every((w) => text.includes(w))
  })
}

/** The steps in runs of the same group, in their order. */
export function groupSteps(steps: StepChoice[]): StepGroup[] {
  const out: StepGroup[] = []
  for (const s of steps) {
    const last = out[out.length - 1]
    if (last && last.name === s.group) last.items.push(s)
    else out.push({ name: s.group, items: [s] })
  }
  return out
}

/** The position after a key press: by delta, wrapping at both ends. */
export function nextIndex(at: number, delta: number, count: number): number {
  if (count <= 0) return 0
  return (at + delta + count) % count
}

/** The step at a position counted across all the groups. */
export function stepAt(groups: StepGroup[], index: number): StepChoice | undefined {
  return groups.flatMap((g) => g.items)[index]
}

/** The list with "Route by status" after Route, when the new step would follow `after`, a step that has a status. */
export function withRouteByStatus(steps: StepChoice[], after: string): StepChoice[] {
  const at = steps.findIndex((s) => s.type === 'condition')
  if (!after || at < 0) return steps
  const shortcut: StepChoice = {
    group: steps[at].group,
    name: 'Route by status',
    description: `Take a path by the status of ${after}: 200, 404, 5xx or else`,
    type: 'condition',
    mode: 'status',
  }
  return [...steps.slice(0, at + 1), shortcut, ...steps.slice(at + 1)]
}
