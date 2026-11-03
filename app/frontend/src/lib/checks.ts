// What the latest send says about each of a request's checks (its assert: lines).
// The engine reports only the checks that went wrong, as notes naming the check, so a check
// no note names passed only if the send got as far as running them, and the check was one of those it was sent with.

export type CheckState = 'idle' | 'passed' | 'failed' | 'error'

/** The part of a ResponseView that tells how its checks went. */
export interface CheckOutcome {
  status: string
  diags: string[]
  /** The checks the send judged. One not in this list was added or edited after it: it has not been tried. */
  checks: string[]
}

const RAN = ['passed', 'failed']

export function checkStates(checks: string[], outcome: CheckOutcome | undefined): CheckState[] {
  return checks.map((check) => checkState(check, outcome))
}

function checkState(check: string, outcome: CheckOutcome | undefined): CheckState {
  const text = check.trim()
  if (!text || !outcome) return 'idle'
  // The response says nothing about a check it was not sent with, so it must not read as passed.
  if (!outcome.checks.some((c) => c.trim() === text)) return 'idle'
  const notes = outcome.diags.filter((d) => d.includes(text))
  if (notes.some((d) => d.includes('assertion failed'))) return 'failed'
  if (notes.length) return 'error'
  return RAN.includes(outcome.status) ? 'passed' : 'idle'
}
