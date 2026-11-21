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

/**
 * Where a broker put a message, as a chip: "p2 @ 10345" for a Kafka partition and offset, "#7" for a JetStream
 * sequence, "" for a protocol with neither. Partition 0 with offset 0 is the very first message, which is real, so it
 * shows when an offset is given by the broker (the bridge sends zero for "none", which cannot be told apart from the
 * first message; a first message shows nothing).
 */
export function positionChip(m: { partition?: number; offset?: number; sequence?: number }): string {
  if (m.offset) return `p${m.partition ?? 0} @ ${m.offset}`
  if (m.sequence) return `#${m.sequence}`
  return ''
}
