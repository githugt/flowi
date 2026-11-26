// The sections of a message request's editor. WebSocket and SSE sign in with an auth block, so they have an Auth tab
// like an HTTP request; the protocols with credential bindings have a Credentials tab; the rest have neither.
// Before: WebSocket and SSE had neither, so the only way to sign in was an Authorization header typed by hand.
import type { RequestView } from './types'

export function asyncSections(view: RequestView): string[] {
  const p = view.async?.block.Protocol
  const signIn = view.async?.auth ? ['Auth'] : view.async?.credentials.length ? ['Credentials'] : []
  return ['Message', 'Headers', 'Receive', ...signIn, 'Binding', 'Variables', 'Extract', 'Assert', 'Outcomes', 'Settings', 'Hooks', 'Schema', 'Docs', 'Code', 'Responses', 'Used by'].filter(
    (s) => !(s === 'Receive' && p === 'sns') && !(s === 'Message' && p === 'sse' && !view.async?.block.Send),
  )
}
