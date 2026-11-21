import { describe, expect, it } from 'vitest'

const files = import.meta.glob('../*.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

describe('empty states', () => {
  // Before: Schedules, Trash, Runs, Stored responses and Blueprints each wrote a bare paragraph ("None yet.",
  // "The trash is empty."), while the rest used EmptyState, so the same situation looked four ways.
  it('use EmptyState, not a bare paragraph that says None or No … yet', () => {
    const bare = /<p class="(?:muted|note)[^"]*">(?:None\b|No [^<]{0,30} yet|The trash is empty)/
    const offenders = Object.entries(files).filter(([, text]) => bare.test(text)).map(([f]) => f)

    expect(Object.keys(files).length).toBeGreaterThan(50) // the glob still finds them
    expect(offenders).toEqual([])
  })
})

// @ts-ignore
import runsTab from '../RunsTab.svelte?raw'

describe('runs tab', () => {
  // Before: a saved run had no link to its trace.
  it('offers Open trace for a run when the trace address is set', () => {
    expect(runsTab).toContain('TracingService.TraceURL')
    expect(runsTab).toMatch(/\{#if traces\[r\.id\]\}[^\n]*Open trace/)
  })
})

// @ts-ignore
import responsePane from '../ResponsePane.svelte?raw'

describe('response facts', () => {
  // Before: the HTTP version, compression and redirects were recorded and shown nowhere.
  it('shows the facts line and the redirect hops', () => {
    expect(responsePane).toContain('r.facts')
    expect(responsePane).toContain('r.hops')
  })
})

describe('run trend', () => {
  // Before: the Runs tab listed runs and added nothing up.
  it('shows the trend line of the latest runs in the Runs tab', () => {
    expect(runsTab).toContain('app.trends[view.node]')
  })
})

describe('empty state learn link', () => {
  it('takes a learn address and shows Learn more for a web address only', async () => {
    // Before: an empty state said what was missing but never pointed to a page that explains it.
    const state = (await import('../EmptyState.svelte?raw')).default as string
    const secrets = (await import('../SecretsScreen.svelte?raw')).default as string

    expect(state).toContain('learn?: string')
    expect(state).toContain('isWebUrl(learn)')
    expect(state).toContain('Learn more')
    expect(secrets).toContain('learn={helpUrl(')
  })
})
