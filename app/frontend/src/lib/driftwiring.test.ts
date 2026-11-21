import { describe, expect, it } from 'vitest'
import tools from '../FlowTools.svelte?raw'

describe('drift check wiring', () => {
  it('should accept one request or all of them, offer the CI command, and keep the result when the dialog closes', () => {
    // Before: the drift check was read-only, its result was lost on close, and accepting meant leaving it.
    expect(tools).toContain('FlowService.AcceptDrift(node, requests)')
    expect(tools).toContain('accept([q.request])')
    expect(tools).toContain('accept([])')
    expect(tools).toContain('copyText(drift?.ci')
    expect(tools).toContain('matrix = access = sequence = null')
  })
})
