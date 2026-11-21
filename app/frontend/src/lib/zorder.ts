// The order cards are painted in: SVG has no z-index, the last one drawn is on top. The selected cards and the one
// being dragged go last, so they sit over a neighbour they overlap. Containers keep their place: raising one would
// cover the cards inside it.
// Before: cards were drawn in layout order, so a card dragged onto another slid under it.

/** `units` with the selected cards after the others and the dragged card last; the order inside each group is kept. */
export function paintOrder<T extends { id: string }>(units: T[], selected: ReadonlySet<string>, movingId: string | undefined, raisable: (u: T) => boolean): T[] {
  const rest: T[] = []
  const chosen: T[] = []
  const dragged: T[] = []
  for (const u of units) {
    if (!raisable(u)) rest.push(u)
    else if (u.id === movingId) dragged.push(u)
    else if (selected.has(u.id)) chosen.push(u)
    else rest.push(u)
  }
  return [...rest, ...chosen, ...dragged]
}
