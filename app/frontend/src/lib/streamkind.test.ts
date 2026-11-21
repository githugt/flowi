import { describe, expect, it } from 'vitest'
import { streamNote } from './streamkind'

describe('streamNote', () => {
  it('Given an unknown method Then it offers waiting for messages', () => {
    expect(streamNote(undefined).reads).toBe(true)
  })
  it('Given a unary method Then it says one reply', () => {
    expect(streamNote({ clientStream: false, serverStream: false }).title).toBe('Unary: one reply')
  })
  it('Given a server-streaming method Then it reads until the stream ends', () => {
    const n = streamNote({ clientStream: false, serverStream: true })
    expect(n.title).toBe('Reads until the stream ends')
    expect(n.reads).toBe(true)
  })
  it('Given a bidirectional method Then it reads until the stream ends and mentions both sides', () => {
    const n = streamNote({ clientStream: true, serverStream: true })
    expect(n.hint).toContain('Both sides')
  })
  it('Given a client-streaming method Then it ends after the last message', () => {
    expect(streamNote({ clientStream: true, serverStream: false }).title).toBe('Ends after your last message')
  })
})
