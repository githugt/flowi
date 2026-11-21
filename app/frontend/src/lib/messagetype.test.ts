import { describe, expect, it } from 'vitest'
import { positionChip, typeChip } from './messagetype'

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

describe('positionChip', () => {
  // Before: the partition, offset and sequence of a broker message never reached the feed.
  it('shows a Kafka partition and offset, or a JetStream sequence', () => {
    expect(positionChip({ partition: 2, offset: 10345 })).toBe('p2 @ 10345')
    expect(positionChip({ sequence: 7 })).toBe('#7')
  })
  it('shows nothing for a protocol that has neither', () => {
    expect(positionChip({})).toBe('')
  })
})
