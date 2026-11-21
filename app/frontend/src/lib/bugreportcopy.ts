// What "Copy for a bug report" tells the person once the text is on the clipboard.
// Before: sharing a request with its response took three manual copies, each with whatever secrets were in it.

/** The toast's words after the copy: what was masked, or that nothing looked like a credential. */
export function bugReportNote(masked: string[]): string {
  if (!masked.length) return 'Copied for a bug report. Nothing looked like a credential: read it before you share it.'
  const shown = masked.slice(0, 4).join(', ') + (masked.length > 4 ? `, and ${masked.length - 4} more` : '')
  return `Copied for a bug report. Masked: ${shown}. Read it before you share it.`
}
