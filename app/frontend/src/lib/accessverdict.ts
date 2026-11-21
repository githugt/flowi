// How one cell of the access-check grid reads. Another person's identity trying a step's resource: being refused is
// the good result, being let in the bad one, and the words say which, so a tick does not read as "the call worked".
// Before: a cell read "✓ 403 denied", and the tick looked like a call that went through.
export type Verdict = 'denied' | 'allowed' | 'unclear'

export function verdictText(verdict: Verdict | string, status: number): { mark: string; text: string; title: string } {
  const code = status ? `${status} ` : ''
  switch (verdict) {
    case 'denied':
      return { mark: '✓', text: `${code}refused, as it should be`, title: 'Another identity was refused: access is protected' }
    case 'allowed':
      return { mark: '✗', text: `${code}let in: not protected`, title: 'Another identity was let in: this resource is open to it' }
    default:
      return { mark: '?', text: `${code}unclear`, title: 'The answer does not show whether access was refused' }
  }
}
