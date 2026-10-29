// Where the Start and End cards sit. They are not steps: the layout of the steps knows nothing of them, so the
// canvas places them beside the first and the last card. Plain functions, so the placing is tested without a window.
import { CARD } from './card'

const WIDTH = 280
const GAP = 80
const PAD = 12

interface Box {
  id: string
  x: number
  y: number
  w: number
  h: number
}

export interface Placed {
  x: number
  y: number
  w: number
  h: number
  /** The End card is only the invitation to add an output: no output exists yet. */
  ghost?: boolean
}

/** The height of a Start or End card with `rows` rows; while editing it has one more, the Add row. */
export function edgeHeight(rows: number, editing: boolean): number {
  return CARD.header + CARD.subline + (rows + (editing ? 1 : 0)) * CARD.row + PAD
}

/** The Start card left of the leftmost step and the End card right of the rightmost, each level with its top. */
export function edgeCards(
  units: Box[],
  inputs: number,
  outputs: number,
  editing: boolean,
): { start: Placed; end: Placed | null; first?: Box; last?: Box } {
  const first = units.reduce<Box | undefined>((a, u) => (!a || u.x < a.x || (u.x === a.x && u.y < a.y) ? u : a), undefined)
  const last = units.reduce<Box | undefined>((a, u) => (!a || u.x + u.w > a.x + a.w ? u : a), undefined)
  const start: Placed = { x: first ? first.x - GAP - WIDTH : 0, y: first ? first.y : 0, w: WIDTH, h: edgeHeight(inputs, editing) }
  if (!last || (outputs === 0 && !editing)) return { start, end: null, first, last }
  const end: Placed = { x: last.x + last.w + GAP, y: last.y, w: WIDTH, h: edgeHeight(outputs, editing), ghost: outputs === 0 }
  return { start, end, first, last }
}

/** The row a dragged row lands on after moving `dy` screen pixels at zoom `k`, kept inside the list. */
export function dropIndex(from: number, dy: number, k: number, count: number): number {
  const rows = Math.round(dy / (CARD.row * k))
  return Math.min(Math.max(from + rows, 0), Math.max(count - 1, 0))
}

/** The types an input or output may declare; "" is any. */
export const IO_TYPES = ['', 'string', 'number', 'boolean', 'object', 'array'] as const
