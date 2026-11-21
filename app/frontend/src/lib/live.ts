import { onEvent, type EventName } from './events'

/**
 * Keeps a view in step with Go: it asks for the state once and then follows
 * the event. The state call can answer after a newer event arrived (Go loads
 * in the background), so a state that comes after an event is dropped.
 * Returns the function that stops following. When the state call fails, `fail` hears of it; without it the failure is
 * left to the caller's own state function to report.
 * Before: a failure was always dropped, so a failed SettingsService.State() left no theme and no message.
 */
export function live<T>(event: EventName, state: () => Promise<unknown>, set: (v: T) => void, fail?: (e: unknown) => void): () => void {
  let seen = false
  const off = onEvent(event, (data) => {
    seen = true
    set(data as T)
  })
  state().then((v) => {
    if (!seen) set(v as T)
  }).catch((e) => fail?.(e))
  return off
}
