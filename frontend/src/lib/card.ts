// Pure helpers of the step card (StepCard.svelte): sizes that must agree with
// internal/canvas/cards.go, colour tokens, and text that fits.

/** Header, sub-line and row heights, in canvas units: keep in step with canvas.cardHeaderH, cardSublineH and cardRowH. */
export const CARD = { header: 46, subline: 28, row: 34, portGap: 24, inPort: 23 } as const

/** The y of port `id` of a card unit, in canvas units. A port the card does not have is the in port's line. */
export function portLine(top: number, ports: string[] | undefined, id: string | undefined): number {
  const i = id && ports ? ports.indexOf(id) : -1
  return top + CARD.inPort + Math.max(i, 0) * CARD.portGap
}

/** Text cut to `max` characters, ending in an ellipsis when it was cut. */
export function clipText(text: string, max: number): string {
  return text.length <= max ? text : text.slice(0, Math.max(1, max - 1)) + '…'
}

/** The colour token of a port: done is green, fail red, a case the accent, else muted. */
export function toneToken(tone: string): string {
  return { done: '--ok', fail: '--fail', case: '--accent', else: '--muted' }[tone] ?? '--muted'
}

export const COLOUR_NAMES = ['blue', 'green', 'amber', 'pink', 'purple', 'teal'] as const

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

/** The icon of a step type for its header tile. */
export function typeIcon(type: string): string {
  const icons: Record<string, string> = {
    request: 'send', subflow: 'blueprint', fetch: 'send', webhook: 'send', set: 'variables', assert: 'assert',
    condition: 'usedby', batch: 'refresh', run_till: 'refresh', parallel: 'list', delay: 'clock',
    log: 'docs', save: 'save', digest: 'chart', stop: 'trash', note: 'docs',
  }
  return icons[type] ?? 'list'
}

/** The icon of a summary row (Face rows name theirs by key). */
export function rowIcon(key: string): string {
  const icons: Record<string, string> = {
    inputs: 'params', send: 'send', receive: 'stream', connection: 'binding', hooks: 'hooks',
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
