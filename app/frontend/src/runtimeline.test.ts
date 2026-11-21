import { describe, expect, it } from 'vitest'

describe('the run timeline panel', () => {
  it('should be offered only for a finished run with steps to compare, and select a step on click', async () => {
    // Before: the flow editor showed one duration per card and no view of when steps ran or overlapped.
    const editor = (await import('./FlowEditor.svelte?raw')).default as string
    const panel = (await import('./RunTimeline.svelte?raw')).default as string

    expect(editor).toContain('!view.run.running && (view.run.timeline?.length ?? 0) > 1')
    expect(editor).toContain('<RunTimeline bars={view.run.timeline} onselect={showStep} />')
    expect(panel).toContain('{#each rows as r (r.step)}')
    expect(panel).toContain('onclick={() => onselect(r.step)}')
    expect(panel).not.toContain('<button')
  })
})
