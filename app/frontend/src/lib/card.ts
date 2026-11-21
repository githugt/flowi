// Pure helpers of the step card (StepCard.svelte): sizes that must agree with
// internal/canvas/cards.go, colour tokens, and text that fits.

/** Header, sub-line and row heights, in canvas units: keep in step with canvas.cardHeaderH, cardSublineH and cardRowH. */
export const CARD = { header: 46, subline: 28, row: 34, portGap: 24, inPort: 23 } as const

/** The y of port `id` of a card unit, in canvas units. A port the card does not have is the in port's line. */
export function portLine(top: number, ports: string[] | undefined, id: string | undefined): number {
  const i = id && ports ? ports.indexOf(id) : -1
  return top + CARD.inPort + Math.max(i, 0) * CARD.portGap
}

/** The colour token of a port: done is green, fail red, a case the accent, else muted. */
export function toneToken(tone: string): string {
  return { done: '--ok', fail: '--fail', case: '--accent', else: '--muted', outcome: '--info' }[tone] ?? '--muted'
}

/** The colour token of a port: the colour its outcome was given, else the one of its tone. */
export function portToken(p: { tone: string; color?: string }): string {
  return p.color && (COLOUR_NAMES as readonly string[]).includes(p.color) ? `--c-${p.color}` : toneToken(p.tone)
}

export const COLOUR_NAMES = ['blue', 'green', 'amber', 'pink', 'purple', 'teal', 'black', 'red', 'sky'] as const

/** The colour an outcome has when it was given none: success green, failure red, any named one aqua blue. */
export const outcomeColour = (name: string, given?: string) => given || (name === 'success' ? 'green' : name === 'failure' ? 'red' : 'sky')

/** The token a step is drawn in: its own colour when set, else its category's. */
export function colourToken(type: string, colour?: string): string {
  if (colour && (COLOUR_NAMES as readonly string[]).includes(colour)) return `--c-${colour}`
  return categoryToken(type)
}

/** A step type's category colour token. */
export function categoryToken(type: string): string {
  if (['request', 'subflow', 'fetch', 'webhook'].includes(type)) return '--request'
  if (['condition', 'batch', 'run_till', 'parallel'].includes(type)) return '--container'
  if (['stop'].includes(type)) return '--fail'
  return '--action'
}

/**
 * The name the add-step popup gives a step type, so a step reads the same where it is added and where it is edited
 * (the file's type is `batch`; the person added a Loop).
 */
export function typeLabel(type: string): string {
  const labels: Record<string, string> = {
    request: 'Request', fetch: 'Request', webhook: 'Request', subflow: 'Flow', set: 'Set value', assert: 'Check',
    condition: 'Route', batch: 'Loop', run_till: 'Loop', parallel: 'Group', delay: 'Wait',
    log: 'Output', save: 'Output', digest: 'Output', stop: 'Stop', note: 'Note',
  }
  return labels[type] ?? type
}

/** The icon of a step type for its header tile. */
export function typeIcon(type: string): string {
  const icons: Record<string, string> = {
    request: 'send', subflow: 'flow', fetch: 'globe', webhook: 'hook', set: 'variables', assert: 'assert',
    condition: 'usedby', batch: 'repeat', run_till: 'loop', parallel: 'list', delay: 'clock',
    log: 'log', save: 'save', digest: 'chart', stop: 'stop', note: 'note',
  }
  return icons[type] ?? 'list'
}

/** The icon of a summary row (Face rows name theirs by key). */
export function rowIcon(key: string): string {
  const icons: Record<string, string> = {
    inputs: 'params', send: 'send', receive: 'stream', connection: 'binding', hooks: 'hooks', outcomes: 'checks',
    settings: 'settings', values: 'variables', checks: 'assert', steps: 'list', message: 'docs',
  }
  return icons[key] ?? 'list'
}

/** The row that is open under a card. */
export interface OpenRow {
  unit: string
  key: string
}

/**
 * What is open after a click on row `key` of card `unit`: that row, unless it is the one already open (it closes),
 * and nothing when there is no editor to show (viewing a flow). One row is open at a time, so opening a row
 * closes the one before it.
 */
export function openAfterClick(current: OpenRow | null, unit: string, key: string, hasPanel: boolean): OpenRow | null {
  if (!hasPanel) return null
  if (current && current.unit === unit && current.key === key) return null
  return { unit, key }
}

/** A tab of the output dock under a card. */
export type DockTab = 'input' | 'output' | 'checks' | 'messages' | 'problems'

/**
 * The tab the dock opens on. Before: always the first one, so a failed step opened on Input and the error was one
 * click away. A step that did not pass opens on Problems, or on Checks when a check failed and nothing else was
 * reported.
 */
export function firstDockTab(tabs: DockTab[], status: string, failedChecks = 0): DockTab | '' {
  if (['failed', 'error', 'aborted'].includes(status)) {
    if (tabs.includes('problems')) return 'problems'
    if (failedChecks > 0 && tabs.includes('checks')) return 'checks'
  }
  return tabs[0] ?? ''
}

/**
 * The tabs the dock offers for what a step did: a request has Input, Output and Checks; a message request its
 * Messages and Checks; any other step its Output. Problems appears when the step reported some. A tab with
 * nothing in it is left out, so the first one is always worth opening.
 */
export function dockTabs(d: { kind: string; input: unknown[]; output: unknown[]; tests: unknown[]; messages: unknown[]; diags: string[] }): DockTab[] {
  const tabs: DockTab[] = []
  if (d.kind === 'messages') {
    if (d.messages.length) tabs.push('messages')
  } else {
    if (d.kind === 'request' && d.input.length) tabs.push('input')
    if (d.output.length) tabs.push('output')
  }
  if (d.tests.length) tabs.push('checks')
  if (d.diags.length) tabs.push('problems')
  return tabs
}
