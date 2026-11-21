import { describe, expect, it } from 'vitest'

describe('the table chips', () => {
  it('should show for data file requests, read again when the path changes, and ask before replacing a query', async () => {
    // Before: a data file request showed no table, sheet or column names, so the person had to know them to write FROM sales.
    const sections = (await import('./DatasetSections.svelte?raw')).default as string
    const chips = (await import('./DataTableChips.svelte?raw')).default as string

    expect(sections).toContain("{#if isFile}{#key block.Driver + '|' + block.Server}<DataTableChips")
    expect(chips).toContain('await RequestService.DataTables(node)')
    expect(chips).toContain('clearTimeout(timer)')
    expect(chips).toContain("ui.confirm('Replace the query?'")
    expect(chips).toContain('{#each tables as t (')
    expect(chips).not.toContain('<button')
  })
})
