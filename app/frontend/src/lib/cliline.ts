// The command line that does what a form in the window does, so a setup tried here can go into a script or CI.
// Before: the window never showed the `flowi serve` command that matched the replayer form, and the form's percent
// became the flag's 0-to-1 share only inside Go.
import { splitList } from './replayer'
import type { ReplayerView } from './types'

/** A word for a POSIX shell: as it is when it is plain, else in single quotes. */
export function shellWord(s: string): string {
  return /^[A-Za-z0-9_@%+=:,./-]+$/.test(s) ? s : `'${s.replace(/'/g, `'\\''`)}'`
}

const flag = (name: string, value: string) => (value.trim() ? [`--${name}`, shellWord(value.trim())] : [])

/** A percent typed in the form (0 to 100) as the share the flag takes (0 to 1); '' when it is blank, zero or not a number. */
export function percentAsShare(text: string): string {
  const n = Number(text.trim())
  return text.trim() && Number.isFinite(n) && n > 0 ? String(Math.min(n, 100) / 100) : ''
}

/** The `flowi serve` command for the replayer form. */
export function serveCommand(f: ReplayerView['form']): string {
  const words = ['flowi', 'serve', '--no-triggers', '--no-schedules']
  words.push(...flag('replay-addr', f.addr), ...flag('latency', f.latency))
  if (f.sequence) words.push('--sequence')
  if (f.cors) words.push('--cors')
  if (f.forward) {
    words.push(...flag('forward', f.forward), ...flag('forward-to', f.forwardTo))
    for (const api of splitList(f.forwardOnly)) words.push(...flag('forward-only', api))
    words.push(...flag('forward-env', f.env))
    if (f.forward === 'save') words.push(...flag('record-to', f.recordTo), ...flag('record-folder', f.recordFolder))
  } else {
    // Faults apply only to calls a saved request answers, which is when nothing is forwarded.
    words.push(...flag('fault-error', percentAsShare(f.faultErrorPercent)), ...flag('fault-status', f.faultStatus), ...flag('fault-drop', percentAsShare(f.faultDropPercent)), ...flag('fault-jitter', f.faultJitter))
  }
  words.push(...flag('record-har', f.recordHar))
  return words.join(' ')
}

/** What a `flowi run` needs to do what the Run button does. */
export interface RunLine {
  /** The flow's saved path and its collection's name. */
  path: string
  collection: string
  /** The selected environment, '' for none. */
  env: string
  /** The input values given (an input sent empty on purpose has the value ''). */
  inputs: Record<string, string>
  /** The last run's seed, so the command repeats its generated values; '' for a fresh one. */
  seed: string
}

/**
 * The `flowi run` command that repeats a run from the window. Before: the window gave no way to take what was tried
 * here into a script or CI, so the command was rebuilt by hand and often differed (another environment, no inputs).
 */
export function runCommand(r: RunLine): string {
  const words = ['flowi', 'run', shellWord(r.path), ...flag('collection', r.collection), ...flag('env', r.env)]
  for (const key of Object.keys(r.inputs).sort()) words.push('--input', shellWord(`${key}=${r.inputs[key]}`))
  words.push(...flag('seed', r.seed))
  return words.join(' ')
}
