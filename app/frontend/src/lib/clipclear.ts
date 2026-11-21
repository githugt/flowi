// Copying a secret leaves it on the clipboard for anything to read. This copies it and clears the clipboard after a
// while, but only if the clipboard still holds the secret (the person may have copied something else since).
// Before: no way to copy a secret, so it was read off the screen and typed or pasted from elsewhere.

export const CLEAR_AFTER_MS = 30_000

export interface ClipboardPort {
  write: (text: string) => Promise<void>
  read: () => Promise<string>
  /** Calls `fn` after `ms`; returns a function that cancels it. */
  later: (fn: () => void, ms: number) => () => void
}

/** Copies `value`, and clears the clipboard after `ms` when it still holds `value`. Returns what cancels the clearing. */
export async function copyAndClear(value: string, port: ClipboardPort, ms = CLEAR_AFTER_MS): Promise<() => void> {
  await port.write(value)
  return port.later(async () => {
    try {
      if ((await port.read()) === value) await port.write('')
    } catch {
      // the clipboard cannot be read: leave it, rather than wipe what may be something else
    }
  }, ms)
}
