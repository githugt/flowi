// Which arrows of the canvas a run actually took, so a finished run shows the path it went.
// Before: every arrow leaving a step that passed was drawn as live, also the failure and route arrows nobody took,
// and the dashes kept moving after the run ended.

const RAN = ['passed', 'running', 'failed', 'error', 'cancelled', 'aborted']
const FAILED = ['failed', 'error', 'aborted']

/**
 * An arrow is taken when the step it leaves ended the way the arrow needs (a failure arrow needs a failure, the
 * others a pass) and the step it leads to ran. An arrow from a step that is still running counts while the target
 * has not started, so a run is seen reaching forward.
 */
export function edgeTaken(edge: { fail?: boolean }, from: string, to: string): boolean {
  const toRan = RAN.includes(to)
  if (edge.fail) return FAILED.includes(from) && toRan
  if (from === 'running') return true
  return from === 'passed' && toRan
}

/** Whether a run is going, so only then do the taken arrows move. */
export const running = (statuses: Record<string, string>): boolean => Object.values(statuses).includes('running')
