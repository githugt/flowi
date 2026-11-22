import { describe, expect, it } from 'vitest'

describe('the steps panel', () => {
  it('should be opened from the More menu or the Runs tab and select a step on click', async () => {
    // Before: a long flow was one very long row with no outline, so a step far along was found by scrolling.
    const editor = (await import('./FlowEditor.svelte?raw')).default as string
    const panel = (await import('./StepsPanel.svelte?raw')).default as string
    const runs = (await import('./RunsTab.svelte?raw')).default as string

    // Before: Steps and Timeline were toolbar buttons and showed only the last run; now a run is chosen in the Runs tab.
    expect(editor).toContain('{#if !embedded && panels.steps === LIVE}<StepsPanel {view} onselect={showStep} />{/if}')
    expect(editor).toContain("label: 'Steps of the last run'")
    expect(editor).not.toContain('>Steps</Button>')
    expect(runs).toContain("toggleRunPanel(view.node, 'steps', r.id)")
    expect(panel).toContain('{#each rows as r (r.id)}')
    expect(panel).toContain('onclick={() => onselect(r.id)}')
    expect(panel).not.toContain('<button')
  })
})
