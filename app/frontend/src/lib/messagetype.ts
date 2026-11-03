// The protocol's own name for a message (an SSE event name, a WebSocket frame kind), as the feed shows and filters it.

/**
 * What to show as the type of a message, or "" when it would add nothing. The default SSE name ("message") is on every
 * unnamed event, and a "text" or "binary" frame says no more than the body's own kind pill already does.
 */
export function typeChip(type: string | undefined, kind: string | undefined): string {
  if (!type || type === 'message') return ''
  const saysNothingNew = (type === 'text' && (kind === 'json' || kind === 'text')) || (type === 'binary' && kind === 'binary')
  return saysNothingNew ? '' : type
}
