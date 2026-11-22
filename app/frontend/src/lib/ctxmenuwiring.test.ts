import { describe, expect, it } from 'vitest'
import app from '../App.svelte?raw'
import json from '../JsonNode.svelte?raw'
import canvas from '../Canvas.svelte?raw'
import flow from '../FlowEditor.svelte?raw'
import inspector from '../Inspector.svelte?raw'
import chip from '../SkipChip.svelte?raw'
import runs from '../RunsTab.svelte?raw'

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
    expect(flow).toMatch(/if \(editing\) \{\s*items\.push\(\{ label: 'Skip this step'[\s\S]*?Pause before this step/)
    expect(flow).toContain("label: 'Delete…'")
  })
  // Before: a skip was offered without Edit and was dropped when the flow was closed; now it is saved with the flow, so it is an edit.
  it('Given a skip is saved with the flow Then only Edit offers to change it', () => {
    expect(inspector).toMatch(/\{#if editing\}\s*<button class="skip"/)
    expect(chip).toContain('{#if editing}<Button small')
    expect(flow).toContain('<SkipChip {view} {editing} />')
  })
})

// Before: the Runs tab could only list saved runs; they could not be cleared from the app.
describe('the Runs tab clears runs', () => {
  it('Given saved runs Then there is a Clear all that asks first, and a right-click that clears one run', () => {
    expect(runs).toContain('FlowService.ClearRuns(view.node)')
    expect(runs).toMatch(/ui\.confirm\('Clear all runs\?'/)
    expect(runs).toContain('oncontextmenu={(e) => runMenu(e, r.id)}')
    expect(runs).toContain("label: 'Clear this run'")
    expect(runs).toContain('FlowService.DeleteRun(view.node, id)')
  })
})
