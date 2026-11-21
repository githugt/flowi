// Keeping the open tabs for the next session without a call to Go for every change to the tab list.
// Before: the effect called SaveTabs on every mutation of the tabs, including each step of a drag reorder and every
// switch of the active tab (which makes a new array of the same ids).
import { debounced } from './debounce'

export interface TabSaver {
  (rows: string[]): void
  /** Saves the waiting list now, for the window closing. */
  flush(): void
}

/** The ids as one comparable string (an id never holds a newline). */
const keyOf = (rows: string[]) => rows.join('\n')

/** A saver that calls `save` once the list has stopped changing for `ms`, and not at all when it equals the last one saved. */
export function createTabSaver(save: (rows: string[]) => void, ms = 300): TabSaver {
  let saved: string | undefined
  const write = debounced((rows: string[]) => {
    const k = keyOf(rows)
    if (k === saved) return
    saved = k
    save(rows)
  }, ms)
  const call = ((rows: string[]) => write([...rows])) as TabSaver
  call.flush = () => write.flush()
  return call
}
