// crashDetails is what "Copy details" puts on the clipboard when a tab fails to draw: enough to report, with no
// file contents from the tab.
// Before: one render error in any editor blanked the whole window, with nothing to copy.
export function crashDetails(error: unknown, tab: string): string {
  const e = error instanceof Error ? error : new Error(String(error))
  const stack = (e.stack ?? '').split('\n').slice(0, 8).join('\n')
  return [`Flowi: the tab "${tab}" failed to display.`, `${e.name}: ${e.message}`, stack].filter(Boolean).join('\n')
}
