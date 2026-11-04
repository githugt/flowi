// What a value trace says, in a line. The trace itself is worked out by the backend (analysis.Trace).
import type { TraceHop, TraceView } from './types'

/** One hop of a run, said briefly: "session made t-1", "verify read t-1", "late read (no value)". */
export function hopText(h: TraceHop): string {
  const verb = h.role === 'made' ? 'made' : 'read'
  return `${h.node} ${verb} ${h.known ? h.value : '(no value)'}`
}

export function traceSummary(t: TraceView): string {
  return `${makers(t)} · ${readers(t)}`
}

function makers(t: TraceView): string {
  if (t.kind === 'input') return 'a flow input'
  if (!t.producers.length) return 'nothing makes it'
  return `${t.generated ? 'generated' : 'made'} by ${t.producers.join(', ')}`
}

function readers(t: TraceView): string {
  return t.readers.length ? `read by ${t.readers.join(', ')}` : 'no step reads it'
}
