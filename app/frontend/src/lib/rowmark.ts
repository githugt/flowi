// What a sidebar row shows after its name: an unsaved dot, and how the last run or send of an open item went.
// Before: a row showed only its name, so unsaved changes and a failed last run were visible only in the tab.
import type { RunView } from './types'

export type RowResult = 'passed' | 'failed' | ''

const BAD = ['failed', 'error', 'aborted']

/** How the last run of a flow went: failed when it ended with an error or any step failed; "" while it runs or before one. */
export function flowResult(run: Pick<RunView, 'running' | 'error' | 'statuses'> | undefined): RowResult {
  if (!run || run.running) return ''
  if (run.error) return 'failed'
  const all = Object.values(run.statuses ?? {})
  if (all.length === 0) return ''
  return all.some((s) => BAD.includes(s)) ? 'failed' : 'passed'
}

/** How the last send of a request went: its checks decide, as the response pane's title does. */
export function requestResult(response: { passed: boolean } | undefined): RowResult {
  if (!response) return ''
  return response.passed ? 'passed' : 'failed'
}
