// What a gRPC method does with its messages, said in the Stream tab.
// Before: the tab said "This request only sends" for every method, also for a server-streaming one that the
// backend reads until the stream ends.
import type { GRPCMethod } from './types'

export type StreamNote = { title: string; hint: string; reads: boolean }

/** The note for a method: the empty state of the Receive section. `m` is undefined while the schema is not known. */
export function streamNote(m: Pick<GRPCMethod, 'clientStream' | 'serverStream'> | undefined): StreamNote {
  if (!m) {
    return { title: 'Not waiting for messages', hint: 'Wait for messages to check the replies or events that follow.', reads: true }
  }
  if (m.serverStream) {
    return {
      title: 'Reads until the stream ends',
      hint: m.clientStream
        ? 'Both sides stream. Set a count or a timeout to stop earlier than the end of the stream.'
        : 'The server sends a stream. Set a count or a timeout to stop earlier than the end of the stream.',
      reads: true,
    }
  }
  if (m.clientStream) {
    return { title: 'Ends after your last message', hint: 'The client sends the messages and the server answers once at the end.', reads: false }
  }
  return { title: 'Unary: one reply', hint: 'The server answers once. The reply is in the response.', reads: false }
}
