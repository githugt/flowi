// What a Ctrl/Cmd+letter press does on the flow canvas: select all steps, copy the selected ones, paste steps.
// Before: the canvas answered only Undo, Save and Delete, so steps could not be selected together or copied.

import { isLetter, type KeyPress } from './keymatch'

export type ClipAction = 'selectAll' | 'copy' | 'paste'

export interface ClipContext {
  /** Focus is in a field, which has its own select all, copy and paste. */
  typing: boolean
  /** The flow can be changed (Edit is on). */
  editing: boolean
  /** Steps are selected. */
  selected: number
  /** Some text on the page is selected: Ctrl+C copies that, as it always did. */
  textSelected: boolean
}

/** The action for a press with Ctrl or Cmd held, or null when the page keeps the key. Select all and paste change the flow. */
export function clipAction(e: KeyPress, c: ClipContext): ClipAction | null {
  if (c.typing) return null
  if (isLetter(e, 'a') && c.editing) return 'selectAll'
  if (isLetter(e, 'c') && c.selected > 0 && !c.textSelected) return 'copy'
  if (isLetter(e, 'v') && c.editing) return 'paste'
  return null
}
