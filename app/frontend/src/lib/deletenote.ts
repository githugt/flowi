// The question before steps are deleted: what goes with them and what will be left reading nothing.
// Before: it said only "Delete the step x?", so a loop with eight steps and a check that read one of them looked small.
import { plural } from './plural'

export interface DeleteImpact {
  steps: number
  readBy: string[]
}

/** The sentence that follows "Delete <what>?": the extra steps that go, and the steps that read what is deleted. */
export function impactNote(selected: number, impact: DeleteImpact | null): string {
  if (!impact) return ''
  const parts: string[] = []
  const inside = impact.steps - selected
  if (inside > 0) parts.push(`${plural(inside, 'step')} inside ${inside === 1 ? 'it goes' : 'them go'} too.`)
  if (impact.readBy.length) parts.push(`${impact.readBy.join(', ')} ${impact.readBy.length === 1 ? 'reads' : 'read'} ${impact.steps === 1 ? 'its' : 'their'} result and will then read nothing.`)
  return parts.join(' ')
}
