import { describe, expect, it } from 'vitest'
import { asyncSections } from './asyncsections'
import type { RequestView } from './types'

const view = (protocol: string, over: { auth?: boolean; credentials?: string[]; send?: boolean }): RequestView =>
  ({ async: { auth: over.auth ?? false, credentials: over.credentials ?? [], block: { Protocol: protocol, Send: over.send ? {} : null } } }) as unknown as RequestView

describe('asyncSections', () => {
  // Before: WebSocket and SSE showed neither Auth nor Credentials.
  it('gives WebSocket and SSE an Auth tab and no Credentials tab', () => {
    for (const p of ['ws', 'sse']) {
      const s = asyncSections(view(p, { auth: true, send: true }))
      expect(s).toContain('Auth')
      expect(s).not.toContain('Credentials')
    }
  })
  it('keeps the Credentials tab of a protocol with credential bindings, and no Auth tab', () => {
    const s = asyncSections(view('mqtt', { credentials: ['username', 'password', 'token'] }))
    expect(s).toContain('Credentials')
    expect(s).not.toContain('Auth')
  })
  it('shows neither for a protocol with no sign-in at all', () => {
    const s = asyncSections(view('sns', {}))
    expect(s).not.toContain('Credentials')
    expect(s).not.toContain('Auth')
  })
  it('hides Message for an SSE request that sends nothing, and Receive for SNS', () => {
    expect(asyncSections(view('sse', { auth: true }))).not.toContain('Message')
    expect(asyncSections(view('sns', {}))).not.toContain('Receive')
  })
})
