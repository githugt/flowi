import { describe, expect, it } from 'vitest'

describe('the run question in a protected environment', () => {
  it('should ask only when the flow writes, and still ask when the check fails', async () => {
    // Before: every run in a protected environment asked the same sentence, even a flow that only reads.
    const src = (await import('./RunMenu.svelte?raw')).default as string
    const at = src.indexOf('async function go(')
    const body = src.slice(at, src.indexOf('async function go2(', at))

    expect(body).toContain('await FlowService.Writes(node)')
    expect(body).toContain('if (lines.length) text = writesQuestion(env, lines)')
    expect(body).toMatch(/catch \{\s*text = writesUnknownQuestion\(env\)/)
    expect(body).toContain('if (text) {')
  })
})
