/**
 * The line of help shown under a step parameter: what it is for, from the step registry. An expression field shows
 * its own help line, so it gets none here.
 * Before: the text showed only as a placeholder, which disappears when the user types.
 */
export function paramDoc(p: { kind: string; doc?: string }): string {
  return p.kind === 'expr' ? '' : (p.doc ?? '').trim()
}

// What a parameter's empty field shows: a value of the right shape, which says more than the help line below it
// (that line is no longer repeated as a placeholder).
const EXAMPLES: Record<string, string> = {
  'batch.over': '{{inputs.users}}',
  'batch.times': '5',
  'batch.concurrency': '4',
  'batch.label': '{{item.email}}',
}

export function paramExample(step: string, name: string): string {
  return EXAMPLES[`${step}.${name}`] ?? ''
}
