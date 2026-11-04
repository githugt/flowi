// Words for the "Suggest flows" review dialog. Pure, so what the dialog says is tested.
import type { FlowSuggestion } from './types'

type Take = FlowSuggestion['steps'][number]['takes'][number]

/** How a link between two steps reads: "petId from create.id". */
export function takeText(t: Take): string {
  return `${t.param} from ${t.from}.${t.field}`
}

/** "90% sure": the confidence as a person would say it. */
export function sureText(confidence: number): string {
  return `${Math.round(confidence * 100)}% sure`
}

/** The least sure link of a suggestion, 1 for a flow that links nothing: the number the list is ordered and judged by. */
export function weakestLink(s: FlowSuggestion): number {
  const all = s.steps.flatMap((step) => step.takes.map((t) => t.confidence))
  return all.length ? Math.min(...all) : 1
}

/** The suggestions with the surest first, so the one most likely to be right is the one at the top. */
export function surestFirst(list: FlowSuggestion[]): FlowSuggestion[] {
  return [...list].sort((a, b) => weakestLink(b) - weakestLink(a) || a.name.localeCompare(b.name))
}

/** A one-line summary of a suggestion's shape: "POST → GET → DELETE". */
export function shapeText(s: FlowSuggestion): string {
  return s.steps.map((step) => step.method).join(' → ')
}

const NODE_W = 64
const NODE_H = 26
const GAP = 34
/** A link below this is drawn as unsure: the person should look at it before accepting. */
const SURE = 0.85

export interface MiniNode {
  id: string
  label: string
  x: number
  y: number
  w: number
  h: number
}
export interface MiniLink {
  from: string
  to: string
  label: string
  weak: boolean
  /** An SVG path: a straight line to the next step, a curve over the steps in between. */
  path: string
}
export interface MiniLayout {
  width: number
  height: number
  nodes: MiniNode[]
  links: MiniLink[]
}

/**
 * A small picture of a suggested flow for the review dialog: the steps in a row, labelled by their verb, and a link
 * from the step a value comes from to the step that takes it. Links over the steps between curve above them, so
 * they do not run through a box.
 */
export function miniLayout(s: FlowSuggestion): MiniLayout {
  const arch = 22 // the room the curves need above the row
  const top = arch
  const nodes = s.steps.map((step, i) => ({ id: step.id, label: step.method, x: i * (NODE_W + GAP), y: top, w: NODE_W, h: NODE_H }))
  const byId = new Map(nodes.map((n, i) => [n.id, { n, i }]))
  const links: MiniLink[] = []
  s.steps.forEach((step, to) => {
    for (const t of step.takes) {
      const source = byId.get(t.from)
      if (!source) continue
      links.push({ from: t.from, to: step.id, label: t.param, weak: t.confidence < SURE, path: linkPath(source.n, nodes[to], to - source.i) })
    }
  })
  const last = nodes[nodes.length - 1]
  return { width: last ? last.x + last.w : 0, height: top + NODE_H, nodes, links }
}

function linkPath(from: MiniNode, to: MiniNode, distance: number): string {
  if (distance === 1) {
    const y = from.y + from.h / 2
    return `M${from.x + from.w} ${y} L${to.x} ${y}`
  }
  const x1 = from.x + from.w / 2
  const x2 = to.x + to.w / 2
  return `M${x1} ${from.y} Q${(x1 + x2) / 2} ${from.y - 2 * 20} ${x2} ${to.y}`
}
