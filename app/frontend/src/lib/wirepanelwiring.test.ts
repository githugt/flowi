import { describe, expect, it } from 'vitest'
import panel from '../WirePanel.svelte?raw'

describe('wire panel preview wiring', () => {
  it('should ask Go for the value after a pause, drop stale answers, and let a click on a child set Take', () => {
    // Before: "Take" was free text with no preview and no clickable tree.
    expect(panel).toContain('FlowService.WirePreview(node, from, path.trim())')
    expect(panel).toContain('setTimeout(')
    expect(panel).toContain('mine === asked')
    expect(panel).toContain('(take = c.path)')
  })
})
