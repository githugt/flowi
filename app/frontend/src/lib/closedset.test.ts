import { describe, expect, it } from 'vitest'
import { ClosedSet } from './closedset'

describe('ClosedSet', () => {
  it('drops events for a closed document until it is opened again', () => {
    const s = new ClosedSet()
    expect(s.accepts('a')).toBe(true)

    s.close('a', 'b')
    expect(s.accepts('a')).toBe(false)
    expect(s.accepts('b')).toBe(false)
    expect(s.accepts('c')).toBe(true)

    s.reopen('a')
    expect(s.accepts('a')).toBe(true)
    expect(s.accepts('b')).toBe(false)
  })
})
