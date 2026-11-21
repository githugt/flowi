const FAILED = new Set(['failed', 'error'])

/**
 * The first step that failed, in the order of the flow (the order of `order`, which lists the step ids as the canvas
 * has them). "" when no step failed. Before, retrying meant: select the failed card, open the Run menu, choose
 * "Run from the selected step".
 */
export function firstFailed(statuses: Record<string, string> | undefined, order: string[]): string {
  if (!statuses) return ''
  const seen = order.find((id) => FAILED.has(statuses[id] ?? ''))
  if (seen) return seen
  // A step the canvas does not list (inside a collapsed group) still counts: take the first by name, for a stable answer.
  return Object.keys(statuses).filter((id) => FAILED.has(statuses[id])).sort()[0] ?? ''
}
