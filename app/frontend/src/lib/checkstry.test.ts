import { describe, expect, it } from 'vitest'

describe('trying a check on the response on screen', () => {
  it('should ask Go for the checks the send did not judge, on a pause, and only for an HTTP request', async () => {
    // Before: a check added after a send stayed a grey circle until the request was sent again, which for a POST made another record.
    const cards = (await import('../AssertCards.svelte?raw')).default as string
    const editor = (await import('../RequestEditor.svelte?raw')).default as string

    expect(cards).toContain("sentStates[i] === 'idle'")
    expect(cards).toContain('debounced(')
    expect(cards).toContain('(not sent again)')
    expect(editor).toContain("evaluate={view.kind === 'http' ? (lines) => RequestService.EvalChecks(node, lines) : undefined}")
  })
})
