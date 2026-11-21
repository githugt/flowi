import { describe, expect, it } from 'vitest'

describe('the run preflight', () => {
  it('should ask before a run that reads what the environment lacks, and never block on a failed check', async () => {
    // Before: a missing variable or secret was found in the middle of a run, after earlier steps had made their requests.
    const src = (await import('./RunMenu.svelte?raw')).default as string
    const at = src.indexOf('async function preflight(')
    const body = src.slice(at, src.indexOf('async function start(', at))

    expect(body).toContain('await FlowService.Preflight(node)')
    expect(body).toMatch(/catch \{[^}]*\}/)
    expect(body).toContain("'Run anyway'")
    expect(src).toContain('await preflight(scope, given)')
  })
})
