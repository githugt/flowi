// The list of toasts on screen: the same message twice is one toast with a count, and there are never more than
// MAX_TOASTS at once.
// Before: a repeating error stacked identical toasts until they ran off the screen.

/** A button on a toast, such as Undo. It is never merged with another toast of the same text. */
export interface ToastAction {
  label: string
  run: () => void
}

export interface Toast {
  id: number
  level: string
  text: string
  action?: ToastAction
  /** How many times the same message arrived while this one was on screen. */
  count?: number
}

export const MAX_TOASTS = 4

/**
 * Adds a toast. `id` is the number a new toast takes. When the same text and level is already shown, that toast is
 * counted again and `reused` says so (its timer starts over); when the list is full the oldest toasts go, and
 * `dropped` names them so their timers can be cleared.
 */
export function addToast(list: Toast[], text: string, level: string, id: number, action?: ToastAction): { list: Toast[]; id: number; reused: boolean; dropped: number[] } {
  // A toast with a button is its own: two deletes of the same name each keep their Undo.
  const same = action ? undefined : list.find((t) => t.text === text && t.level === level && !t.action)
  if (same) {
    return { list: list.map((t) => (t === same ? { ...t, count: (t.count ?? 1) + 1 } : t)), id: same.id, reused: true, dropped: [] }
  }
  const next = [...list, action ? { id, level, text, action } : { id, level, text }]
  const over = Math.max(0, next.length - MAX_TOASTS)
  return { list: next.slice(over), id, reused: false, dropped: next.slice(0, over).map((t) => t.id) }
}
