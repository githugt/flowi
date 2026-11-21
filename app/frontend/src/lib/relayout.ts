// The words of the Re-layout menu item and the note after it.
// Before: the item said only "Re-layout", and whether it moved the selection or every step (throwing away hand
// placement) depended on something the label did not show.
import { plural } from './plural'

/** The menu label: what it will move. */
export function relayoutLabel(selected: number): string {
  return selected > 0 ? `Re-layout ${plural(selected, 'selected step')}` : 'Re-layout all steps'
}

/** The note shown once it is done; Undo brings the old places back. */
export function relayoutNote(selected: number, total: number): string {
  const n = selected > 0 ? selected : total
  return `Re-laid out ${plural(n, 'step')}. Undo brings the old places back.`
}
