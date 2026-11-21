// Deleting the selected steps of a flow, from the Delete key or the inspector's button. It asks first:
// a step can hold settings and links that are tedious to rebuild, and the key did it silently.
import { FlowService } from '../../bindings/github.com/githugt/flowi/app/internal/bridge'
import { ui } from './ui.svelte'
import { impactNote } from './deletenote'

/** Asks, then deletes the selected steps of the flow open as `node`. The question says what else goes or breaks. */
export async function confirmDeleteSteps(node: string, selection: string[]) {
  if (!selection.length) return
  const what = selection.length === 1 ? `the step ${selection[0]}` : `${selection.length} steps (${selection.join(', ')})`
  const impact = await FlowService.DeleteImpact(node, selection).catch(() => null) // without it the question is the plain one
  const note = impactNote(selection.length, impact)
  ui.confirm(selection.length === 1 ? 'Delete step' : 'Delete steps', `Delete ${what}?${note ? ' ' + note : ''} Undo brings it back.`, () => FlowService.DeleteSelected(node), 'Delete')
}
