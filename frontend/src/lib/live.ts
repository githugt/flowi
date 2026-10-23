import { Events } from '@wailsio/runtime'

/**
 * Keeps a view in step with Go: it asks for the state once and then follows
 * the event. The state call can answer after a newer event arrived (Go loads
 * in the background), so a state that comes after an event is dropped.
 * Returns the function that stops following.
 */
export function live<T>(event: string, state: () => Promise<unknown>, set: (v: T) => void): () => void {
  let seen = false
  const off = Events.On(event, (e: any) => {
    seen = true
    set(e.data as T)
  })
  state().then((v) => {
    if (!seen) set(v as T)
  })
  return off
}
