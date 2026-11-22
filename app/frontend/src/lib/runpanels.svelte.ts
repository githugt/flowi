// Which run the Steps list and the Timeline of an open flow show. The two panels sit under the flow's toolbar, and are
// opened from two places: the Runs tab, for any saved run, and the toolbar's More menu, for the run that just finished.
// The tab and the editor are different components, so what is open is kept here, by node.
// Before: both were toolbar buttons that could only show the last run.

/** The run that was just made in this window, as opposed to a saved one picked in the Runs tab. */
export const LIVE = 'live'

export type RunPanel = 'steps' | 'timeline'

export interface RunPanels {
  /** The run each panel shows: LIVE, a saved run's id, or null while the panel is closed. */
  steps: string | null
  timeline: string | null
}

const state = $state<Record<string, RunPanels>>({})

/** What a node has open. */
export function runPanelsOf(node: string): RunPanels {
  return state[node] ?? { steps: null, timeline: null }
}

/** Opens a panel on a run, or closes it when it already shows that run. */
export function toggleRunPanel(node: string, panel: RunPanel, run: string) {
  const now = runPanelsOf(node)
  state[node] = { ...now, [panel]: now[panel] === run ? null : run }
}

/** Closes a panel. */
export function closeRunPanel(node: string, panel: RunPanel) {
  state[node] = { ...runPanelsOf(node), [panel]: null }
}
