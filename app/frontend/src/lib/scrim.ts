/** The part of a form field that tells whether the person changed it. */
export type Field = { value: string; checked?: boolean }

/** What the fields hold now, as text. */
export const fieldValues = (fields: ArrayLike<Field>): string[] =>
  Array.from(fields, (f) => `${f.value}|${f.checked ?? ''}`)

/** Whether the fields differ from the snapshot taken when the dialog opened. */
export function isDirty(before: string[], fields: ArrayLike<Field>): boolean {
  const now = fieldValues(fields)
  return now.length !== before.length || now.some((v, i) => v !== before[i])
}

/**
 * Svelte action for the dim layer behind a dialog: a press on the layer itself closes the dialog, unless the person
 * changed a field in it. Before: any press outside threw away what they had typed.
 */
export type ScrimClose = (() => void) | { onclose: () => void; /** False for a layer whose field is a query, not a form: a press outside closes it even when it was typed in. */ guard?: boolean }
const closer = (a: ScrimClose) => (typeof a === 'function' ? a : a.onclose)
const guarded = (a: ScrimClose) => typeof a === 'function' || a.guard !== false

export function scrimClose(node: HTMLElement, arg: ScrimClose) {
  let close = closer(arg)
  let guard = guarded(arg)
  let before: string[] = []
  const fields = () => node.querySelectorAll<HTMLInputElement>('input, textarea, select')
  // Let the dialog set its start values first, then remember them.
  const timer = setTimeout(() => (before = fieldValues(fields())), 0)
  const down = (e: PointerEvent) => {
    if (e.target !== node) return
    if (guard && isDirty(before, fields())) return
    close()
  }
  node.addEventListener('pointerdown', down)
  return {
    update(next: ScrimClose) {
      close = closer(next)
      guard = guarded(next)
    },
    destroy() {
      clearTimeout(timer)
      node.removeEventListener('pointerdown', down)
    },
  }
}
