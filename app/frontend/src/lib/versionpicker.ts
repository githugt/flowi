// The one picker for a request's version and the overlays laid over it: the choices, their order and what
// a click changes. Stacking works as before: overlays apply in the order named and a later one wins.

export interface PickItem {
  group: 'Versions' | 'Overlays'
  id: string // a version's label, an overlay's name, "" for None
  label: string
  on: boolean
}

export interface PickInput {
  versions: string[]
  names: string[] // a name for each version, "" for none
  def: string // the default version
  selected: string
  overlays: string[] // every overlay the collection has
  stack: string[] // the ones laid over the open version, in order
}

/** The versions, then None and the overlays; what is open is marked. */
export function pickerItems(p: PickInput): PickItem[] {
  const versions = p.versions.map<PickItem>((v, i) => ({
    group: 'Versions',
    id: v,
    label: v + (p.names[i] ? ' · ' + p.names[i] : '') + (v === p.def ? ' (default)' : ''),
    on: v === p.selected,
  }))
  const overlays = p.overlays.map<PickItem>((o) => ({ group: 'Overlays', id: o, label: o, on: p.stack.includes(o) }))
  return [...versions, { group: 'Overlays', id: '', label: 'None', on: p.stack.length === 0 }, ...overlays]
}

/** The stack after a click on an overlay: in when it was out (last), out when it was in; None clears it. */
export function toggleOverlay(stack: string[], name: string): string {
  if (!name) return ''
  return (stack.includes(name) ? stack.filter((n) => n !== name) : [...stack, name]).join('+')
}

/** What the picker's button says: the version, then each overlay. */
export function pickerLabel(selected: string, stack: string[]): string {
  return [selected, ...stack].join(' + ')
}
