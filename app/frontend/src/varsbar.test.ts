import { describe, expect, it } from 'vitest'

describe('the variables bar', () => {
  it('should list the steps that write and read a variable and show one on click', async () => {
    // Before: the bar listed names only, so finding who changed a variable meant reading every step.
    const bar = (await import('./VarsBar.svelte?raw')).default as string
    const editor = (await import('./FlowEditor.svelte?raw')).default as string

    expect(bar).toContain('Written by')
    expect(bar).toContain('Read by')
    expect(bar).toContain('onclick={() => onselect?.(id)}')
    expect(bar).toContain('(id)}')
    expect(editor).toContain('<VarsBar vars={view.vars} onselect={showStep} />')
  })
})
