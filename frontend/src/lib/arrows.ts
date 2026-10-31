// The arrow between two cards says when the second runs: on which outcomes of the first (success, failure, or
// any named outcome of its request). The layout draws one edge per outcome, so an arrow taken more than one
// way is several edges between the same cards.
import { COLOUR_NAMES } from './card'
import type { Edge, Face } from './types'

/** What a picked arrow stands for: the cards it joins, and whether it is an on_error goto (not an arrow written on the step). */
export const arrowKey = (e: Pick<Edge, 'from' | 'to' | 'goto'>) => `${e.from}>${e.to}${e.goto ? '>goto' : ''}`

/** The outcomes that take the arrow between cards from and to, success and failure first, then the names in the order drawn. */
export function arrowOutcomes(edges: Edge[], from: string, to: string): string[] {
  const between = edges.filter((e) => !e.into && e.from === from && e.to === to && !e.goto)
  const out: string[] = []
  if (between.some((e) => !e.fail && !e.outcome)) out.push('success')
  if (between.some((e) => e.fail && !e.outcome)) out.push('failure')
  for (const e of between) if (e.outcome && !out.includes(e.outcome)) out.push(e.outcome)
  return out
}

/** The outcomes after the person switches one on or off. */
export const toggled = (outcomes: string[], outcome: string) => (outcomes.includes(outcome) ? outcomes.filter((o) => o !== outcome) : [...outcomes, outcome])

/** The edges that make up the picked arrow (every outcome of it). */
export const arrowEdges = (edges: Edge[], key: string) => edges.filter((e) => !e.into && arrowKey(e) === key)

/** The outcome a port stands for: done is success, fail is failure, outcome-x is x; '' for a port that is not one (a case). */
export function outcomeOfPort(port: string): string {
  if (port === 'done') return 'success'
  if (port === 'fail') return 'failure'
  return port.startsWith('outcome-') ? port.slice('outcome-'.length) : ''
}

/** The outcomes a card's arrows can follow, in the order of its ports; a card without ports of its own has success and failure. */
export function outcomesOfCard(face: Face | undefined): string[] {
  const named = (face?.ports ?? []).map((p) => outcomeOfPort(p.id)).filter(Boolean)
  return named.length ? named : ['success', 'failure']
}

/**
 * The id of the marker an arrow ends in: a ring in the arrow's colour. Each colour has its own marker, since a marker
 * cannot be told to take the colour of the line it is on (context-stroke is not drawn by every web view).
 */
export function arrowMarker(e: { fail?: boolean; outcome?: string }, colour?: string): string {
  if (colour && (COLOUR_NAMES as readonly string[]).includes(colour)) return `arrow-c-${colour}`
  if (e.fail) return 'arrow-fail'
  return e.outcome ? 'arrow-c-sky' : 'arrow'
}
