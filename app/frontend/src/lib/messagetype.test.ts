import { describe, expect, it } from 'vitest'
import { typeChip } from './messagetype'

describe('typeChip', () => {
  it('shows an event name', () => {
    expect(typeChip('order.paid', 'json')).toBe('order.paid')
  })

  it('shows nothing for a message with no type, or the default SSE name', () => {
    expect(typeChip(undefined, 'json')).toBe('')
    expect(typeChip('message', 'text')).toBe('')
  })

  it('shows nothing for a frame kind the body kind already says', () => {
    expect(typeChip('text', 'json')).toBe('')
    expect(typeChip('text', 'text')).toBe('')
    expect(typeChip('binary', 'binary')).toBe('')
  })

  it('shows a frame kind that disagrees with the body', () => {
    // A binary frame whose bytes happen to be text: the body pill says text, the protocol says binary.
    expect(typeChip('binary', 'text')).toBe('binary')
  })
})
