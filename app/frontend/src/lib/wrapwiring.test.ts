import { describe, expect, it } from 'vitest'
import editor from '../FlowEditor.svelte?raw'
import bar from '../WrapBar.svelte?raw'

describe('wrap bar wiring', () => {
  it('should sit over the canvas for a multi-selection and call the bridge, showing a refusal as a message', () => {
    // Before: selecting several steps offered only Delete.
    expect(editor).toContain('showWrapBar(editing, view.selection.length)')
    expect(editor).toContain('<WrapBar {node} count={view.selection.length} />')
    expect(bar).toContain('FlowService.Wrap(node, kind).catch(ui.fail)')
  })
})
