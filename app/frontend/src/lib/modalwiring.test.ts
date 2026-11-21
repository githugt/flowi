import { describe, expect, it } from 'vitest'

const files = import.meta.glob('../*.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>
const named = (name: string) => files[`../${name}.svelte`]

describe('dialogs are modal', () => {
  it('Given every component that draws a scrim Then it is wrapped in the modal action', () => {
    // Before: no dialog trapped focus, said aria-modal, or gave focus back.
    const bare = Object.entries(files)
      .filter(([, src]) => /class="scrim"/.test(src) && !/use:modal/.test(src))
      .map(([path]) => path)

    expect(bare).toEqual([])
  })
  it.each(['Popover'])('Given the popover %s Then the dialog itself has the modal action', (name) => {
    expect(named(name)).toMatch(/use:modal\s+role="dialog"/)
  })
  it.each(['CaseEditor', 'IOEditor', 'StepPopup', 'WirePanel'])('Given %s Then it is a Popover, which has the modal action', (name) => {
    expect(named(name)).toContain('<Popover')
  })
  it.each(['FlowEditor', 'Canvas', 'RequestEditor'])('Given %s Then its window key handler stands down while a dialog is open', (name) => {
    // Before: Delete, Esc and Ctrl+Enter still worked behind an open dialog.
    expect(named(name)).toContain('if (modalOpen()) return')
  })
})
