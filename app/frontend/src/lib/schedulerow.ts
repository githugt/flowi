import { untilText } from './until'

export interface ScheduleRowLike {
  disabled: boolean
  flowLabel: string
  nextAt: string
}

export interface RowStatus {
  text: string
  /** true when the row has a real next run to show on the strip */
  firing: boolean
}

/**
 * What a schedule row says about its next run. Before, the row said "Next run in 3 h" although the runner was off or
 * no flow matched, and once the time had passed it read "Next run " with nothing after it.
 */
export function scheduleStatus(s: ScheduleRowLike, runnerOn: boolean, now: number): RowStatus {
  if (s.disabled) return { text: 'Disabled', firing: false }
  if (!s.flowLabel) return { text: 'Flow not found: it will not run', firing: false }
  if (!s.nextAt) return { text: 'Never fires', firing: false }
  if (!runnerOn) return { text: 'Paused: the schedules are not firing', firing: false }
  const until = untilText(s.nextAt, now)
  return { text: until ? `Next run ${until}` : 'Due now', firing: true }
}
