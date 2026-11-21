// Saving once a person pauses typing instead of on every keystroke.
// Before: editing a collection variable rewrote flowi.yaml on every keystroke, and the "comments will be lost" question
// could appear in the middle of a word.

export interface Debounced<A extends unknown[]> {
  /** Runs fn with these arguments after the quiet period; a call inside it replaces the one waiting. */
  (...args: A): void
  /** Runs the waiting call now, if there is one. For leaving the screen with an edit not yet saved. */
  flush(): void
  /** Drops the waiting call. */
  cancel(): void
}

export function debounced<A extends unknown[]>(fn: (...args: A) => void, ms: number): Debounced<A> {
  let timer: ReturnType<typeof setTimeout> | undefined
  let waiting: A | undefined
  const run = () => {
    clearTimeout(timer)
    timer = undefined
    const args = waiting
    waiting = undefined
    if (args) fn(...args)
  }
  const call = ((...args: A) => {
    waiting = args
    clearTimeout(timer)
    timer = setTimeout(run, ms)
  }) as Debounced<A>
  call.flush = run
  call.cancel = () => {
    clearTimeout(timer)
    timer = undefined
    waiting = undefined
  }
  return call
}
