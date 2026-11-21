/** Statuses that count as a run that went wrong. */
const BAD = ['failed', 'error', 'aborted']

/**
 * The run to compare the newest one with: the newest run that passed, when the newest itself went wrong. Runs are
 * newest first. -1 when there is nothing to compare (no runs, the newest passed or was only cancelled, or no run ever
 * passed). Before: "what changed since it last worked" meant picking two runs by hand.
 */
export function lastGreenIndex(statuses: readonly string[]): number {
  if (!statuses.length || !BAD.includes(statuses[0])) return -1
  return statuses.findIndex((s, i) => i > 0 && s === 'passed')
}
