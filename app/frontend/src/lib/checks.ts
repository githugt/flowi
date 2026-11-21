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
  /** What the send itself said about each check it ran, in order; empty when it did not say (a message, a dataset). */
  checkResults?: { line: string; verdict: string; note: string }[]
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
  const said = outcome.checkResults?.find((r) => r.line.trim() === text)
  if (said) return said.verdict === 'passed' ? 'passed' : said.verdict === 'failed' ? 'failed' : 'error'
  // The send reported its checks, and this one is not among them: it did not get as far as running it.
  if (outcome.checkResults?.length) return 'idle'
  const notes = checkNotes(check, outcome)
  if (notes.some((d) => d.includes('assertion failed'))) return 'failed'
  if (notes.length) return 'error'
  return RAN.includes(outcome.status) ? 'passed' : 'idle'
}

const WORD = /[A-Za-z0-9_.]/

// A note names a check when the check's text stands alone in it: "status == 200" is not named by a note about
// "status == 2000" or "x.status == 200". Before, any note that held the text as a substring counted.
function names(note: string, text: string): boolean {
  for (let at = note.indexOf(text); at >= 0; at = note.indexOf(text, at + 1)) {
    const before = at > 0 ? note[at - 1] : ''
    const after = note[at + text.length] ?? ''
    if ((!before || !WORD.test(before)) && (!after || !WORD.test(after))) return true
  }
  return false
}

/** The notes of a response that are about this check: the reason it failed or could not be evaluated. */
export function checkNotes(check: string, outcome: Pick<CheckOutcome, 'diags'> | undefined): string[] {
  const text = check.trim()
  if (!text || !outcome) return []
  return outcome.diags.filter((d) => names(d, text))
}

/** What trying a check on the response already on screen (no send) said; the shape of the Go CheckEvalView. */
export interface CheckTry {
  line: string
  verdict: 'passed' | 'failed' | 'unknown' | string
  note: string
}

export interface TriedState {
  state: CheckState
  /** True when the state comes from trying the check on the shown response, not from the send. */
  tried: boolean
  /** Why it failed (what it saw) or why it could not be tried; "" when there is nothing to add. */
  note: string
}

/**
 * Each check's state: the send's verdict, or for a check the send did not judge (added or edited since) the
 * verdict of trying it on the response on screen. A check that cannot be tried without sending (it reads a
 * variable or the environment) stays untried, with the reason as its note.
 */
export function triedStates(checks: string[], states: CheckState[], tried: CheckTry[]): TriedState[] {
  return checks.map((check, i) => {
    const text = check.trim()
    const t = states[i] === 'idle' && text ? tried.find((x) => x.line.trim() === text) : undefined
    if (!t) return { state: states[i], tried: false, note: '' }
    if (t.verdict === 'passed') return { state: 'passed', tried: true, note: '' }
    if (t.verdict === 'failed') return { state: 'failed', tried: true, note: t.note }
    return { state: 'idle', tried: false, note: t.note }
  })
}

/** How many of the checks a send ran passed: "3/4 checks". Nothing when the send did not report any. */
export function checkTally(outcome: Pick<CheckOutcome, 'checkResults'> | undefined): { passed: number; total: number; text: string; ok: boolean } | undefined {
  const results = outcome?.checkResults ?? []
  if (!results.length) return undefined
  const passed = results.filter((r) => r.verdict === 'passed').length
  return { passed, total: results.length, text: `${passed}/${results.length} ${results.length === 1 ? 'check' : 'checks'}`, ok: passed === results.length }
}
