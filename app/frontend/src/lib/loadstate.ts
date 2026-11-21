// Where a screen's data is: asked for, here, or refused. A screen that reads this cannot show a false empty state
// ("No secret is declared yet") for a load that failed or has not finished.
// Before: live() dropped every load error, so a cancelled passphrase left a screen that said nothing was there.
import { errorText } from './errors'

export type LoadState = { status: 'loading' } | { status: 'ready' } | { status: 'error'; error: string }

export const LOADING: LoadState = { status: 'loading' }
export const READY: LoadState = { status: 'ready' }

/** The state after a load failed: the message in the window's words. */
export function failed(e: unknown): LoadState {
  return { status: 'error', error: errorText(e) }
}

/** Whether the empty state may be shown: only once the data has really come, and is empty. */
export function mayShowEmpty(state: LoadState, count: number): boolean {
  return state.status === 'ready' && count === 0
}
