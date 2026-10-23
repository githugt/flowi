// Dragging a tab or a row to a new place: where among the others it goes.

export interface Span {
  start: number
  end: number
}

/**
 * Where a dragged item goes: the number of the other items (the dragged one
 * left out) whose middle is before pos. Picking the item under the pointer
 * instead swapped back and forth: after a swap with a wider or narrower
 * neighbour the pointer is still over that neighbour, which asks to swap again.
 */
export function slotAmong(others: Span[], pos: number): number {
  return others.filter((s) => (s.start + s.end) / 2 < pos).length
}

/**
 * Where the drop marker for `slot` (from slotAmong) stands: just before the
 * item now in that slot, or just after the last one. With no other items it
 * is `fallback`.
 */
export function markerAt(others: Span[], slot: number, fallback: number): number {
  if (!others.length) return fallback
  if (slot < others.length) return others[slot].start - 1
  return others[others.length - 1].end + 1
}

/** The list with the item at `from` moved to index `to`. */
export function moved<T>(list: T[], from: number, to: number): T[] {
  if (from === to || from < 0 || to < 0 || from >= list.length || to >= list.length) return list
  const out = [...list]
  const [item] = out.splice(from, 1)
  out.splice(to, 0, item)
  return out
}
