import { describe, expect, it } from 'vitest'
import editor from '../RequestEditor.svelte?raw'

describe('the Expected status field of the Assert tab', () => {
  it('Given an http request Then the Assert tab offers it and saves through SetExpect', () => {
    const at = editor.indexOf("section === 'Assert'")
    const block = editor.slice(at, editor.indexOf("section === 'Outcomes'"))

    expect(block).toContain("view.kind === 'http'")
    expect(block).toContain('aria-label="Expected status"')
    expect(block).toContain('RequestService.SetExpect(node, e.currentTarget.value)')
  })

  it('Given text the server refused Then the problem is shown as an alert', () => {
    expect(editor).toMatch(/\{#if expectProblem\}<span class="expect-problem" role="alert">/)
  })
})
