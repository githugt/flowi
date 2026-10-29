// The kinds that do one job in several ways: a request can be saved, a URL call or a webhook; a loop runs over a list
// or until something holds; an output goes to the log, a file or a digest. The step panel offers them as a
// choice and converts the step; `type` and `mode` are what the bridge's ConvertStep takes.
export interface ModeOption {
  label: string
  node: string // the node type this option makes
  type: string
  mode: string
}

const FAMILIES: ModeOption[][] = [
  [
    { label: 'Saved request', node: 'request', type: 'request', mode: '' },
    { label: 'URL call', node: 'fetch', type: 'request', mode: 'url' },
    { label: 'Webhook', node: 'webhook', type: 'request', mode: 'send' },
  ],
  [
    { label: 'Over a list or N times', node: 'batch', type: 'batch', mode: '' },
    { label: 'Until something holds', node: 'run_till', type: 'batch', mode: 'until' },
  ],
  [
    { label: 'Run log', node: 'log', type: 'log', mode: '' },
    { label: 'File', node: 'save', type: 'log', mode: 'file' },
    { label: 'Digest', node: 'digest', type: 'log', mode: 'digest' },
  ],
]

/** The ways a step of node type `node` can be done, empty when it has only one. */
export function modesOf(node: string): ModeOption[] {
  return FAMILIES.find((f) => f.some((o) => o.node === node)) ?? []
}

/** The sentence that asks before a conversion drops fields; "" when nothing is lost. */
export function lostText(lost: string[]): string {
  if (!lost.length) return ''
  return `These do not carry over and will be removed: ${lost.join(', ')}. Undo brings them back.`
}
