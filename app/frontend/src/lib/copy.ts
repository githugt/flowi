// Copying text to the clipboard, one way for the whole window: try each writer in turn, and say "Copied" only when
// one of them worked.
// Before: three different ways existed (the Wails clipboard, the browser's, and a fire-and-forget call), and a
// copy that failed still told the user "Copied".

export type Writer = (text: string) => void | Promise<void>

export type CopyResult = { ok: true } | { ok: false; error: string }

/** Writes text with the first writer that does not throw. */
export async function copyText(text: string, writers: Writer[]): Promise<CopyResult> {
  let last = 'No way to reach the clipboard here.'
  for (const write of writers) {
    try {
      await write(text)
      return { ok: true }
    } catch (e) {
      last = e instanceof Error ? e.message : String(e)
    }
  }
  return { ok: false, error: last }
}

/** What the toast says: "Copied the body", or why it did not work and what to do instead. */
export function copyMessage(what: string, result: CopyResult): { text: string; level: 'info' | 'error' } {
  if (result.ok) return { text: what ? `Copied ${what}` : 'Copied', level: 'info' }
  return { text: `Could not copy${what ? ` ${what}` : ''}: ${result.error} Select the text and copy it by hand.`, level: 'error' }
}

export type Reader = () => string | Promise<string>

/** Reads text with the first reader that does not throw; null when none could (the clipboard is closed to the page). */
export async function readClipboard(readers: Reader[]): Promise<string | null> {
  for (const read of readers) {
    try {
      return await read()
    } catch {
      // The next reader may be allowed.
    }
  }
  return null
}
