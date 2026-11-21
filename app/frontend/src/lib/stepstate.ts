/** The mark a step card shows for how the step went: a tick, a cross, a dash, a ring or nothing. */
export type StepMark = 'tick' | 'cross' | 'dash' | 'ring' | 'none'

export function stepMark(status: string | undefined): StepMark {
  switch (status) {
    case 'passed':
      return 'tick'
    case 'failed':
    case 'error':
    case 'aborted':
      return 'cross'
    // Before: a skipped or cancelled step fell through to the ring, so a cancelled run looked unfinished.
    case 'cancelled':
    case 'skipped':
      return 'dash'
    case 'running':
      return 'ring'
    default:
      return 'none'
  }
}
