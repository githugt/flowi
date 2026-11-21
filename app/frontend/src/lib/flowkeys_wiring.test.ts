import { describe, expect, it } from 'vitest'
import editor from '../FlowEditor.svelte?raw'

describe('FlowEditor clipboard keys', () => {
  it('Given the canvas When Ctrl+A, C or V is pressed Then the flow service selects, copies and pastes', () => {
    // Before: the canvas answered only Undo, Save and Delete.
    expect(editor).toContain('clipAction(e,')
    expect(editor).toContain('FlowService.SelectAll(node)')
    expect(editor).toContain('FlowService.CopySteps(node, view.selection)')
    expect(editor).toContain('FlowService.PasteSteps(node, text)')
  })
  it('Given the help text When read Then it names the keys', () => {
    expect(editor).toContain('Ctrl+A')
    expect(editor).toContain('Ctrl+C and Ctrl+V')
  })
})
