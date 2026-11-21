import { describe, expect, it } from 'vitest'
import tools from '../FlowTools.svelte?raw'

describe('kept access check wiring', () => {
  it('should bring the last check back, with its form, when the dialog opens', () => {
    // Before: the table was lost when the dialog closed.
    expect(tools).toContain('FlowService.LastAccess(node)')
    expect(tools).toContain("if (t === 'access') await restoreAccess()")
    expect(tools).toContain('owner = c.owner')
  })

  it('should say when the check ran and mark an old one', () => {
    expect(tools).toContain('checkedText(access.checked, new Date())')
    expect(tools).toContain('isStale(access.checked, new Date())')
  })
})
