// The window's one way to copy: the native clipboard first, the browser's second, and a toast that tells the truth.
import { Clipboard } from '@wailsio/runtime'
import { copyMessage, copyText, readClipboard } from './copy'
import { ui } from './ui.svelte'

/**
 * Copies text. `what` completes the toast: "the body" gives "Copied the body". `saidOnSuccess` replaces the success
 * toast with the caller's own words (a failure keeps the usual one). Returns whether it worked.
 */
export async function copy(text: string, what = '', saidOnSuccess = ''): Promise<boolean> {
  const result = await copyText(text, [(t) => Clipboard.SetText(t), (t) => navigator.clipboard.writeText(t)])
  const message = copyMessage(what, result)
  ui.toast(result.ok && saidOnSuccess ? saidOnSuccess : message.text, message.level)
  return result.ok
}

/** The clipboard's text, or null when the window may not read it. */
export function paste(): Promise<string | null> {
  return readClipboard([() => Clipboard.Text(), () => navigator.clipboard.readText()])
}
