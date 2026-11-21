import { errorText } from './errors'

/**
 * Waits for a call to Go and says how it went: a failure is shown as its message, a success as `ok` when given.
 * Returns whether the call worked, so a form can keep what the user typed when it did not.
 * Before: manager screens sent a save and cleared the form at once, so a failure was silent (or only a console line)
 * and the typed text was gone.
 */
export async function reported(call: Promise<unknown>, toast: (text: string, level?: 'error') => void, ok = ''): Promise<boolean> {
  try {
    await call
  } catch (e) {
    toast(errorText(e) || 'Something went wrong.', 'error')
    return false
  }
  if (ok) toast(ok)
  return true
}
