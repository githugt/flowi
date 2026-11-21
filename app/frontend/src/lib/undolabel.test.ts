import { describe, expect, it } from 'vitest'
import editor from '../FlowEditor.svelte?raw'
import types from './types.ts?raw'

import { withShortcut } from './shortcuttip'

describe('undo and redo labels', () => {
  it('shows what Undo and Redo would do as the button tooltip', () => {
    // Before: the buttons showed only on or off; the text comes from Go (vm.DescribeChange).
    expect(editor).toContain('withShortcut(view.undoLabel')
    expect(editor).toContain('withShortcut(view.redoLabel')
    expect(types).toContain('undoLabel: string')
    expect(types).toContain('redoLabel: string')
  })

  it('names the shortcut in the tooltip, for the platform', () => {
    // Before: Undo and Redo were text buttons with no icon and no hint of the keys.
    expect(withShortcut('Undo: add step a', 'Ctrl+Z', false)).toBe('Undo: add step a (Ctrl+Z)')
    expect(withShortcut('Undo', 'Ctrl+Z', true)).toBe('Undo (⌘Z)')
    expect(withShortcut('Redo', 'Ctrl+Shift+Z', true)).toBe('Redo (⇧⌘Z)')
    expect(editor).toContain('<IconButton icon="undo"')
    expect(editor).toContain("variant={view.dirty ? 'primary' : 'secondary'}")
  })
})
