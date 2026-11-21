import { FlowService } from '../../bindings/github.com/githugt/flowi/app/internal/bridge'
import { app } from './app.svelte'
import { ui } from './ui.svelte'
import { errorText } from './errors'

/** Opens the request or flow that a step of an open flow calls, in its own tab. */
export async function openStepTarget(flowNode: string, step: string): Promise<void> {
  try {
    const id = await FlowService.TargetOf(flowNode, step)
    const n = id ? app.find(id) : undefined
    if (n) await app.open(n)
    else ui.toast(id ? 'That request or flow is not in the open collections.' : 'This step does not call a request or a flow.', 'error')
  } catch (e) {
    ui.toast(errorText(e), 'error')
  }
}
