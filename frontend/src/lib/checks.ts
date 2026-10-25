// What the latest send says about each of a request's checks (its assert: lines).
// The engine reports only the checks that went wrong, as notes naming the check, so a check
// no note names passed only if the send got as far as running them.

export type CheckState = 'idle' | 'passed' | 'failed' | 'error'

/** The part of a ResponseView that tells how its checks went. */
export interface CheckOutcome {
  status: string
  diags: string[]
}

const RAN = ['passed', 'failed']

export function checkStates(checks: string[], outcome: CheckOutcome | undefined): CheckState[] {
  return checks.map((check) => checkState(check, outcome))
}

function checkState(check: string, outcome: CheckOutcome | undefined): CheckState {
  const text = check.trim()
  if (!text || !outcome) return 'idle'
  const notes = outcome.diags.filter((d) => d.includes(text))
  if (notes.some((d) => d.includes('assertion failed'))) return 'failed'
  if (notes.length) return 'error'
  return RAN.includes(outcome.status) ? 'passed' : 'idle'
}
