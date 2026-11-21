/**
 * Waits until `done` is true, checking every `stepMs`, and gives up after `timeoutMs`. It tells whether `done` became
 * true. A save call returns before the file is written (the write runs on a worker and the result comes as an event),
 * so a caller that must act on the result waits for the state to change.
 */
export async function settle(done: () => boolean, timeoutMs = 5000, stepMs = 25): Promise<boolean> {
  const end = Date.now() + timeoutMs
  while (!done()) {
    if (Date.now() >= end) return false
    await new Promise((r) => setTimeout(r, stepMs))
  }
  return true
}
