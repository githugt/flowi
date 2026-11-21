// What a screen reader is told when a setting is applied or refused. Settings apply the moment a field changes or
// loses focus, so nothing else says it worked. Before: a refusal showed a red line a screen reader never read.
import { labelOf } from './settings'

interface Snapshot {
  name: string
  value: string
  problem?: string
}

/** "Saved: Theme" or "Not saved: Timeout: not a duration", for the first setting that changed between two views; '' for none. */
export function settingAnnouncement(before: Snapshot[], after: Snapshot[]): string {
  if (before.length === 0) return '' // the first view is the load, not a change
  const was = new Map(before.map((f) => [f.name, f]))
  for (const f of after) {
    const old = was.get(f.name)
    if (!old) continue
    if (f.problem && f.problem !== old.problem) return `Not saved: ${labelOf(f.name)}: ${f.problem}`
    if (f.value !== old.value) return `Saved: ${labelOf(f.name)}`
  }
  return ''
}
