import { describe, expect, it } from 'vitest'

describe('switching a request row off', () => {
  it('should offer the checkbox for parameters and for headers a request sends, and nowhere else', async () => {
    // Before: a header or parameter could only be deleted to try the request without it.
    const rows = (await import('./KVRows.svelte?raw')).default as string
    const editor = (await import('./RequestEditor.svelte?raw')).default as string

    expect(rows).toContain('row.Disabled = !v')
    expect(rows).toContain('{#if toggle}')
    expect(editor).toMatch(/rows=\{v\.Query\} describe toggle /)
    expect(editor).toContain('describe toggle={!sendHeaders} keyLabel="header"')
    // Variables, options and the other tables keep no checkbox: only a request's query and headers know "disabled".
    expect(editor.match(/ toggle[ =]/g)?.length).toBe(2)
  })
})
