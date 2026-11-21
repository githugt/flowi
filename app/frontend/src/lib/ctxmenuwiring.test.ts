import { describe, expect, it } from 'vitest'
import app from '../App.svelte?raw'
import json from '../JsonNode.svelte?raw'
import canvas from '../Canvas.svelte?raw'
import flow from '../FlowEditor.svelte?raw'

describe('context menus of tabs and JSON rows', () => {
  it('Given a tab When right-clicked Then it offers the close choices and the folder', () => {
    // Before: only the sidebar rows had a context menu.
    expect(app).toContain('oncontextmenu={(e) => tabMenu(e, t.node)}')
    expect(app).toContain("label: 'Close others'")
    expect(app).toContain("label: 'Close to the right'")
    expect(app).toContain('WorkspaceService.RevealLocation(node)')
    expect(app).toContain('closePlan(nodes, node, mode')
  })
  it('Given a JSON row When right-clicked Then it offers to copy the value and the path', () => {
    expect(json).toContain('oncontextmenu={menu}')
    expect(json).toContain("label: 'Copy value'")
    expect(json).toContain("label: 'Copy path'")
  })
  // Before: a right-click on a flow's card did nothing the app answered, so the web view's own menu (Inspect Element) opened.
  it('Given a flow card When right-clicked Then it selects the card and offers open, skip and, when editing, the edits', () => {
    expect(canvas).toContain('oncontextmenu={(e) =>')
    expect(canvas).toContain('if (e.button === 2) return')
    expect(flow).toContain('oncontext={stepMenu}')
    expect(flow).toContain("'Open request' : 'Open flow'")
    expect(flow).toContain("label: 'Skip in the next runs'")
    expect(flow).toMatch(/if \(editing\) \{[^}]*Pause before this step/s)
    expect(flow).toContain("label: 'Delete…'")
  })
})
