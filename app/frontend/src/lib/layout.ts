// How the editor is divided into panes: a tree of rows and columns whose leaves each show one tab. Pure, so the
// arrangements people make (a tab dropped on an edge, a pane closed, a layout put back) are tested without a window.
// A split gives its panes equal shares until a divider is dragged; then `sizes` holds the shares (fractions of 1).

export type Side = 'left' | 'right' | 'top' | 'bottom'
export type Dir = 'row' | 'col'

export interface Pane {
  kind: 'pane'
  id: string
  /** The tab it shows. */
  node: string
}
export interface Split {
  kind: 'split'
  dir: Dir
  children: Layout[]
  /** The share of each child, adding up to 1; absent when they are equal. */
  sizes?: number[]
}
export type Layout = Pane | Split

export const leaf = (id: string, node: string): Pane => ({ kind: 'pane', id, node })

/** The least share a pane keeps when a divider is dragged, so a pane cannot be squeezed out of sight. */
export const MIN_SHARE = 0.1

/** The share of each child of a split: `sizes`, or equal shares when none were set. */
export function sharesOf(s: Split): number[] {
  const n = s.children.length
  return s.sizes && s.sizes.length === n ? s.sizes : Array<number>(n).fill(1 / n)
}

interface Part {
  child: Layout
  weight: number
}

/**
 * A split of the parts, tidied: no parts is nothing, one part is that part, and a part that is a split running the
 * same way joins this one (its panes keep their room: the part's weight shared out by its own shares). Weights are
 * made shares; equal shares are not stored.
 */
function assemble(dir: Dir, parts: Part[]): Layout | undefined {
  const flat = parts.flatMap(({ child, weight }) =>
    child.kind === 'split' && child.dir === dir ? sharesOf(child).map((w, i) => ({ child: child.children[i], weight: weight * w })) : [{ child, weight }],
  )
  if (flat.length === 0) return undefined
  if (flat.length === 1) return flat[0].child
  const total = flat.reduce((sum, p) => sum + p.weight, 0)
  const sizes = flat.map((p) => p.weight / total)
  const equal = sizes.every((w) => Math.abs(w - 1 / sizes.length) < 1e-9)
  return { kind: 'split', dir, children: flat.map((p) => p.child), ...(equal ? {} : { sizes }) }
}

/** The panes, in reading order: left to right, top to bottom. */
export function panesOf(l: Layout): Pane[] {
  return l.kind === 'pane' ? [l] : l.children.flatMap(panesOf)
}

/** The tabs the panes show, in reading order. */
export const nodesOf = (l: Layout): string[] => panesOf(l).map((p) => p.node)

/** The layout with pane id showing another tab. */
export function withNode(l: Layout, id: string, node: string): Layout {
  if (l.kind === 'pane') return l.id === id ? { ...l, node } : l
  return { ...l, children: l.children.map((c) => withNode(c, id, node)) }
}

const dirOf: Record<Side, Dir> = { left: 'row', right: 'row', top: 'col', bottom: 'col' }
const comesFirst = (side: Side) => side === 'left' || side === 'top'

/**
 * The layout with a new pane, showing node, on a side of pane id. Beside a pane of a split that runs the same way it
 * joins that split, so three panes in a row are one row and not a row inside a row. A pane that is not there changes
 * nothing.
 */
export function splitPane(l: Layout, id: string, node: string, side: Side, newId: string): Layout {
  return splitIn(l, id, leaf(newId, node), side)
}

function splitIn(l: Layout, id: string, fresh: Pane, side: Side): Layout {
  if (l.kind === 'pane') return l.id === id ? { kind: 'split', dir: dirOf[side], children: comesFirst(side) ? [fresh, l] : [l, fresh] } : l
  const at = l.children.findIndex((c) => c.kind === 'pane' && c.id === id)
  if (at >= 0 && l.dir === dirOf[side]) {
    // The new pane takes an equal share and the others give up the same fraction of theirs.
    const parts: Part[] = l.children.map((child, i) => ({ child, weight: sharesOf(l)[i] * (l.children.length / (l.children.length + 1)) }))
    parts.splice(comesFirst(side) ? at : at + 1, 0, { child: fresh, weight: 1 / (l.children.length + 1) })
    return assemble(l.dir, parts) ?? l
  }
  return { ...l, children: l.children.map((c) => splitIn(c, id, fresh, side)) }
}

/** The layout without pane id: the panes beside it share its room. The last pane is never closed. */
export function closePane(l: Layout, id: string): Layout {
  if (panesOf(l).length <= 1) return l
  return removeFrom(l, id) ?? l
}

function removeFrom(l: Layout, id: string): Layout | undefined {
  if (l.kind === 'pane') return l.id === id ? undefined : l
  const shares = sharesOf(l)
  const parts = l.children.map((c, i) => ({ child: removeFrom(c, id), weight: shares[i] })).filter((p): p is Part => p.child !== undefined)
  return assemble(l.dir, parts)
}

/** The layout after the tab node was closed: the panes that showed it close, and the last one shows fallback instead. */
export function closeNode(l: Layout, node: string, fallback: string): Layout {
  let out = l
  for (const p of panesOf(l)) {
    if (p.node !== node) continue
    out = panesOf(out).length > 1 ? closePane(out, p.id) : withNode(out, p.id, fallback)
  }
  return out
}

export interface Rect {
  left: number
  right: number
  top: number
  bottom: number
}

/** The share of a pane, at each edge, where a dropped tab chooses a side. The middle stays neutral, so a drop that is
 *  not meant to split does not. */
const EDGE = 0.3

/** Which side of a pane a tab dragged to (x, y) would take: the nearest edge, when it is near enough, else none. */
export function edgeOf(x: number, y: number, pane: Rect, edge = EDGE): Side | null {
  const w = pane.right - pane.left
  const h = pane.bottom - pane.top
  if (w <= 0 || h <= 0 || x < pane.left || x > pane.right || y < pane.top || y > pane.bottom) return null
  const distances: [Side, number][] = [
    ['left', (x - pane.left) / w],
    ['right', (pane.right - x) / w],
    ['top', (y - pane.top) / h],
    ['bottom', (pane.bottom - y) / h],
  ]
  const [side, nearest] = distances.reduce((a, b) => (b[1] < a[1] ? b : a))
  return nearest <= edge ? side : null
}

/** A layout as plain data, for storage. */
export function encodeLayout(l: Layout): unknown {
  return l.kind === 'pane' ? { node: l.node } : { dir: l.dir, children: l.children.map(encodeLayout), ...(l.sizes ? { sizes: l.sizes } : {}) }
}

/**
 * A layout read back from storage. Panes whose tab is no longer open (isOpen) are left out and the rest share their
 * room; null when nothing is left, or when the data is not a layout at all: what storage holds can be anything.
 */
export function decodeLayout(data: unknown, isOpen: (node: string) => boolean, newId: () => string): Layout | null {
  return decodeNode(data, isOpen, newId) ?? null
}

function decodeNode(data: unknown, isOpen: (node: string) => boolean, newId: () => string): Layout | undefined {
  if (!data || typeof data !== 'object' || Array.isArray(data)) return undefined
  const o = data as Record<string, unknown>
  if (typeof o.node === 'string') return isOpen(o.node) ? leaf(newId(), o.node) : undefined
  if ((o.dir !== 'row' && o.dir !== 'col') || !Array.isArray(o.children)) return undefined
  const stored = validShares(o.sizes, o.children.length)
  const parts = o.children
    .map((c, i) => ({ child: decodeNode(c, isOpen, newId), weight: stored?.[i] ?? 1 }))
    .filter((p): p is Part => p.child !== undefined)
  return assemble(o.dir, parts)
}

// Stored shares are used only when there is one positive number for each child: storage can hold anything.
function validShares(data: unknown, count: number): number[] | undefined {
  if (!Array.isArray(data) || data.length !== count) return undefined
  return data.every((w) => typeof w === 'number' && Number.isFinite(w) && w > 0) ? (data as number[]) : undefined
}

/**
 * The layout with one divider of a split moved. `path` names the split by the child index taken at each level from
 * the root ([] is the root itself); `boundary` is the divider after that child; `position` is where the pointer is, as
 * a fraction of the split's whole extent. The two panes beside the divider trade room and no other pane changes; each
 * keeps at least `min` of the split. A path or divider that is not there changes nothing.
 */
export function resizeSplit(l: Layout, path: number[], boundary: number, position: number, min = MIN_SHARE): Layout {
  if (l.kind === 'pane') return l
  if (path.length > 0) {
    const [at, ...rest] = path
    if (!l.children[at]) return l
    const children = l.children.map((c, i) => (i === at ? resizeSplit(c, rest, boundary, position, min) : c))
    return { ...l, children }
  }
  if (boundary < 0 || boundary >= l.children.length - 1 || !Number.isFinite(position)) return l
  const shares = [...sharesOf(l)]
  const before = shares.slice(0, boundary).reduce((a, b) => a + b, 0)
  const pair = shares[boundary] + shares[boundary + 1]
  const first = Math.min(Math.max(position - before, min), pair - min)
  if (pair < 2 * min) return l
  shares[boundary] = first
  shares[boundary + 1] = pair - first
  return { ...l, sizes: shares }
}

/**
 * The arrangement a tab dropped on a side of pane `target` makes: the tab in a new pane there, and `target` keeping
 * what it showed. A tab that another pane shows moves from there. Dropped on the pane that shows it, the tab brings the
 * next tab of the strip that no pane shows along (the one before it when it is the last), to fill the pane it leaves.
 * null when the tab is not open, the pane is not there, or there is no other tab to show.
 */
export function splitForDrop(l: Layout, tabs: string[], target: string, node: string, side: Side, newId: string): { layout: Layout; pane: string } | null {
  const pane = panesOf(l).find((p) => p.id === target)
  if (!pane || !tabs.includes(node)) return null
  let layout = l
  if (pane.node === node) {
    const other = tabToFill(tabs, node, nodesOf(l))
    if (!other) return null
    layout = withNode(layout, pane.id, other)
  } else {
    const old = panesOf(layout).find((p) => p.node === node)
    if (old) layout = closePane(layout, old.id)
  }
  return { layout: splitPane(layout, target, node, side, newId), pane: newId }
}

function tabToFill(tabs: string[], node: string, shown: string[]): string | undefined {
  const at = tabs.indexOf(node)
  const hidden = tabs.filter((t, i) => i !== at && !shown.includes(t))
  return hidden.find((t) => tabs.indexOf(t) > at) ?? hidden[hidden.length - 1]
}
