// What is running right now, for the footer: sends in flight and flows running, across every open tab.
// Before: the footer showed the replayer and schedules but nothing said a flow or a send was still going while its
// tab was behind another.

export interface RunChip {
  text: string
  title: string
}

/** The chip for the work in progress, or null when nothing is. */
export function runChip(flows: { run: { running: boolean } }[], requests: { sending: boolean }[]): RunChip | null {
  const f = flows.filter((v) => v.run.running).length
  const r = requests.filter((v) => v.sending).length
  if (f + r === 0) return null
  const parts = [f ? `${f} ${f === 1 ? 'flow' : 'flows'}` : '', r ? `${r} ${r === 1 ? 'send' : 'sends'}` : ''].filter(Boolean)
  return { text: `Running ${parts.join(' · ')}`, title: `In progress in the open tabs: ${parts.join(' and ')}.` }
}
