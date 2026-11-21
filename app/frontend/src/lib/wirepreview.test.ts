import { describe, expect, it } from 'vitest'
import { previewLine, wireRef } from './wirepreview'
import type { WirePreview } from './types'

const preview = (p: Partial<WirePreview>): WirePreview => ({ ran: true, found: true, text: '', children: [], ...p })

describe('wireRef', () => {
  it('should write the template that reads the step, or the flow input', () => {
    // Before: "Take" was free text with no sign of what it would write or pass.
    expect(wireRef('login', 'body.token')).toBe('{{nodes.login.body.token}}')
    expect(wireRef('login', '  ')).toBe('{{nodes.login}}')
    expect(wireRef('', 'email')).toBe('{{inputs.email}}')
  })
})

describe('previewLine', () => {
  it('should show nothing before the first answer', () => {
    expect(previewLine(null)).toEqual({ kind: 'none', text: '' })
  })

  it('should say to run the flow when the step has not run, and when the path is not in the last run', () => {
    expect(previewLine(preview({ ran: false, found: false })).kind).toBe('norun')
    expect(previewLine(preview({ found: false })).kind).toBe('missing')
  })

  it('should show the value, naming an empty one', () => {
    expect(previewLine(preview({ text: 'eyJhbGc' }))).toEqual({ kind: 'value', text: 'eyJhbGc' })
    expect(previewLine(preview({ text: '' })).text).toBe('(empty text)')
  })
})
