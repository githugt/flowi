// What the toast says after steps are pasted: how many, and which requests or flows they call that this collection does
// not have (the steps are there all the same, still pointing at them).
// Before: a paste said only "Pasted 3 steps", so steps that called nothing here looked fine until a run failed.

const plural = (n: number, one: string, many: string) => (n === 1 ? one : many)

export function pasteNote(steps: number, unmatched: string[]): string {
  const done = `Pasted ${steps} ${plural(steps, 'step', 'steps')}`
  if (unmatched.length === 0) return done
  const shown = unmatched.length > 3 ? `${unmatched.slice(0, 3).join(', ')} and ${unmatched.length - 3} more` : unmatched.join(', ')
  return `${done}. This collection has no ${shown}: choose ${plural(unmatched.length, 'another request or flow', 'other requests or flows')} for ${plural(unmatched.length, 'that step', 'those steps')}.`
}
