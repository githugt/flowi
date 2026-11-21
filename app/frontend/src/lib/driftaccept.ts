import type { DriftView } from './types'

type Row = DriftView['requests'][number]

/** Whether a request of the check has anything to accept: differences, or a response with no schema to start from. */
export const canAccept = (q: Row): boolean => !q.accepted && (q.differences.length > 0 || q.noSchema)

/** The requests still to accept, by path, for "Accept all". */
export const pending = (d: DriftView): string[] => d.requests.filter(canAccept).map((q) => q.request)

/** The label of a request's button: a response with no schema learns one, the rest accept what moved. */
export const acceptLabel = (q: Row): string => (q.noSchema && !q.differences.length ? 'Learn schema' : 'Accept')

/** The words after the check passes or fails, which say what accepting changed. */
export function settledNote(d: DriftView): string {
  const done = d.requests.filter((q) => q.accepted).length
  if (!done) return ''
  return `${done} ${done === 1 ? 'request' : 'requests'} accepted: ${done === 1 ? 'its' : 'their'} stored schema now matches what the check saw.`
}
