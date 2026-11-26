import { describe, expect, it } from 'vitest'
import { headersHint } from './headershint'

describe('headersHint', () => {
  // Before: the connection hint said to type the credential as an Authorization header, because WebSocket and SSE had
  // no Auth tab. Both have one now, and the hint points to it like the HTTP one does.
  it('points an HTTP request to its Auth tab', () => {
    expect(headersHint('http')).toContain('go in Auth')
  })
  it('points a connection request to its Auth tab too', () => {
    expect(headersHint('connect')).toContain('go in Auth')
  })
  it('keeps the hint of the message headers', () => {
    expect(headersHint('send')).toBe('Headers travel with each message sent.')
  })
})
