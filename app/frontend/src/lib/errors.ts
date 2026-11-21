// Failed calls to Go. Most calls are not awaited (a click sends an edit and
// the view comes back as an event), so an error Go returns would otherwise
// only reach the console; reportUnhandled shows it to the user instead.

/** Errors of cancelled calls: the window asked for them, so they need no notice. */
const CANCELLED = ['CancelError', 'CancelledRejectionError']

/** The text to show for a failed call: the error's message, without the "Error: " String() adds. */
export function errorText(e: unknown): string {
  if (e instanceof Error) return e.message
  return String(e).replace(/^Error: /, '')
}

/** Whether e only says a call was cancelled. */
export function isCancelled(e: unknown): boolean {
  return e instanceof Error && CANCELLED.includes(e.name)
}

/** Shows the error of a call nobody awaited; cancellations stay quiet. */
export function reportUnhandled(reason: unknown, toast: (text: string, level: string) => void) {
  if (isCancelled(reason)) return
  const text = errorText(reason)
  toast(text || 'Something went wrong.', 'error')
}

/** What the window tells the log about a caught failure; cancellations are not failures. */
export function frontendReport(kind: 'error' | 'rejection', reason: unknown): { kind: string; message: string; stack: string } | null {
  if (isCancelled(reason)) return null
  const stack = reason instanceof Error ? (reason.stack ?? '') : ''
  return { kind, message: errorText(reason), stack }
}

export const REPORT_LIMIT = 20

/** Whether another report may go to the log: a render that fails in a loop must not fill it. */
export function mayReport(sent: number): boolean {
  return sent < REPORT_LIMIT
}
