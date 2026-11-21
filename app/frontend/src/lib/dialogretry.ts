// The dialog that comes back when its action failed: the same dialog, the text the person typed and the reason.
// Before: a prompt closed before its action ran, so a failure (a name already taken, a path that cannot be
// written) lost the typed text and a toast was the only word on what went wrong.
type Retryable = { kind: 'prompt' | 'confirm'; initial?: string; error?: string }

/** The dialog to show again after a failed answer: with the error, and with the typed text as its start. */
export function reopened<T extends Retryable>(d: T, text: string, error: string): T {
  return d.kind === 'prompt' ? { ...d, initial: text, error } : { ...d, error }
}
