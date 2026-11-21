import { plural } from './plural'
import type { SpecUpdate } from './types'

/** The heading of what the new spec would break, or what was found when nothing does; empty when nothing was checked. */
export function breakHeading(p: SpecUpdate): string {
  if (!p.checked) return ''
  return p.breakages.length ? `Would break (${p.breakages.length})` : 'Checked: nothing in your flows breaks'
}

/** The label of the button that applies the update: it says so when the person is updating past breakages. */
export function updateLabel(p: SpecUpdate): string {
  return p.breakages.length ? `Update anyway (${plural(p.breakages.length, 'break')})` : 'Update'
}
