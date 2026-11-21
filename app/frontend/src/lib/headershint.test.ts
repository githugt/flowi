import { describe, expect, it } from 'vitest'
import { headersHint } from './headershint'

describe('headersHint', () => {
  // Before: every list that was not "send" said "Credentials go in Auth", also on WebSocket and SSE, which have no Auth tab.
  it('points an HTTP request to its Auth tab', () => {
    expect(headersHint('http')).toContain('go in Auth')
  })
  it('tells a connection request to put the credential in a header, and never names an Auth tab', () => {
    const hint = headersHint('connect')
    expect(hint).toContain('Authorization')
    expect(hint).not.toContain('go in Auth')
  })
  it('keeps the hint of the message headers', () => {
    expect(headersHint('send')).toBe('Headers travel with each message sent.')
  })
})
