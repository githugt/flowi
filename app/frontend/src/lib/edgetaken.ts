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
  // A skipped step does not hold up the ones after it: it takes its success arrows (lib/engine arrowTaken). One
  // that never started because something before it failed has a target that did not run either.
  return (from === 'passed' || from === 'skipped') && toRan
}

/**
 * The status an arrow reads of a step. A step skipped on purpose (the flow marks it, or the person did: its card is
 * hatched) lets the run through, so for its arrows it counts as passed: the arrow into it is taken like the one out of it.
 */
export const throughStatus = (status: string, skippedOnPurpose: boolean): string => (status === 'skipped' && skippedOnPurpose ? 'passed' : status)

/** Whether a run is going, so only then do the taken arrows move. */
export const running = (statuses: Record<string, string>): boolean => Object.values(statuses).includes('running')
