// Deleting the selected steps of a flow, from the Delete key or the inspector's button. It asks first:
// a step can hold settings and links that are tedious to rebuild, and the key did it silently.
import { FlowService } from '../../bindings/github.com/githugt/flowi/app/internal/bridge'
import { ui } from './ui.svelte'

/** Asks, then deletes the selected steps of the flow open as `node`. */
export function confirmDeleteSteps(node: string, selection: string[]) {
  if (!selection.length) return
  const what = selection.length === 1 ? `the step ${selection[0]}` : `${selection.length} steps (${selection.join(', ')})`
  ui.confirm(selection.length === 1 ? 'Delete step' : 'Delete steps', `Delete ${what}? Undo brings it back.`, () => FlowService.DeleteSelected(node), 'Delete')
}
