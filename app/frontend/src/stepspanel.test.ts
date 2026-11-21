import { describe, expect, it } from 'vitest'

describe('the steps panel', () => {
  it('should be a toggle in the flow editor and select a step on click', async () => {
    // Before: a long flow was one very long row with no outline, so a step far along was found by scrolling.
    const editor = (await import('./FlowEditor.svelte?raw')).default as string
    const panel = (await import('./StepsPanel.svelte?raw')).default as string

    expect(editor).toContain('{#if !embedded && stepsOpen}<StepsPanel {view} onselect={showStep} />{/if}')
    expect(panel).toContain('{#each rows as r (r.id)}')
    expect(panel).toContain('onclick={() => onselect(r.id)}')
    expect(panel).not.toContain('<button')
  })
})
