// A status is a colour plus a shape, so it still reads without colour (colour blindness, forced colours, print).
// Before: the run list marked a run with a coloured dot only.
const MARKS: Record<string, string> = { passed: '✓', failed: '✗', error: '!', aborted: '■', cancelled: '■', running: '…', skipped: '–' }

/** The shape that goes with a status; "?" for one the window does not know. */
export const statusMark = (status: string): string => MARKS[status] ?? '?'
