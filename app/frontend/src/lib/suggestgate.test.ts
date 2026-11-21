import { describe, expect, it } from 'vitest'
import field from '../TemplateField.svelte?raw'
import { inTemplate } from './exprcomplete'

describe('template suggestions', () => {
  it('are asked of Go only while the caret is inside an unclosed {{', () => {
    // Before: every key typed in an address or a value made a call to Go that answered nothing.
    expect(field).toContain('if (suggest && inTemplate(local, caret)) return')
    expect(field).not.toContain('(!vocabulary || inTemplate(local, caret))')
  })
  it('know where a template is open', () => {
    expect(inTemplate('https://x/{{ho', 14)).toBe(true)
    expect(inTemplate('https://x/{{host}}/a', 20)).toBe(false)
    expect(inTemplate('plain', 5)).toBe(false)
  })
})
