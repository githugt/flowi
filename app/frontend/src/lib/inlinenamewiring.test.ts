import { describe, expect, it } from 'vitest'
import inspector from '../Inspector.svelte?raw'
import inline from '../InlineName.svelte?raw'

describe('inline name wiring', () => {
  it('should rename a step from its id field, on Enter or when the field is left', () => {
    // Before: a new id typed in the field was lost when the user clicked away.
    expect(inspector).toContain('<InlineName label="Id')
    expect(inspector).toContain('FlowService.Rename(node, to)')
    expect(inline).toContain('onblur={() => editing && void keep()}')
    expect(inline).toContain("a === 'revert'")
  })

  it('should say a refusal under the field and keep the typed text', () => {
    expect(inline).toContain('{error}')
    expect(inline).toMatch(/catch \(e\) \{\s*error = errorText\(e\)/)
  })
})
