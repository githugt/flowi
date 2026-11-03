// The hooks timeline as cards: each section heading ("Pre-hooks", "Post-hooks", "After the post-hooks") starts a
// card with the steps under it. The request itself has no heading, so it stands in a card of its own.
export interface PipelineStep {
  kind: string
  title: string
  state: string
  [key: string]: unknown
}
export interface PipelineGroup<T extends PipelineStep = PipelineStep> {
  title: string
  steps: T[]
  // How the card as a whole went: every step passed, one failed, one is running, or it has not run (or is off).
  tone: 'passed' | 'failed' | 'running' | 'off' | ''
}

export function toneOf(steps: PipelineStep[]): PipelineGroup['tone'] {
  if (steps.some((s) => s.state === 'failed')) return 'failed'
  if (steps.some((s) => s.state === 'running')) return 'running'
  if (steps.length && steps.every((s) => s.state === 'passed')) return 'passed'
  if (steps.length && steps.every((s) => s.state === 'not-run')) return 'off'
  return ''
}

export function groupPipeline<T extends PipelineStep>(rows: T[]): PipelineGroup<T>[] {
  const groups: { title: string; steps: T[] }[] = []
  for (const row of rows) {
    if (row.kind === 'section') groups.push({ title: row.title, steps: [] })
    else if (row.kind === 'api' || !groups.length) groups.push({ title: '', steps: [row] })
    else if (groups[groups.length - 1].title === '') groups.push({ title: '', steps: [row] })
    else groups[groups.length - 1].steps.push(row)
  }
  return groups.filter((g) => g.steps.length).map((g) => ({ ...g, tone: toneOf(g.steps) }))
}

const RAN = ['passed', 'failed', 'running']

// Where the run's line ends, given each visible step's state in order: at the step that failed, else at the last that
// ran. Steps after that never ran, so there is nothing to join them to. Before any send nothing has run, and the line
// shows the whole way (ran is false).
export function flowEnd(states: string[]): { end: number; ran: boolean } {
  const failed = states.indexOf('failed')
  const lastRan = states.findLastIndex((s) => RAN.includes(s))
  const end = failed >= 0 ? failed : lastRan >= 0 ? lastRan : states.length - 1
  return { end, ran: lastRan >= 0 }
}
