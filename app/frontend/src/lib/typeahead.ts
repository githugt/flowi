// Keys that move through a list of rows by name or by end: Home, End, and typing the start of a name.
// Before: the sidebar tree answered the arrow keys only, so reaching "users" in a long list took many presses.

export const TYPE_AHEAD_MS = 700

/** How long the sidebar's Jump field waits after a keystroke before it filters the tree. */
export const JUMP_DELAY_MS = 80

export interface TypeAhead {
  buffer: string
  at: number
}

/** A key that types a character into the search: one printable character, no Ctrl, Alt or Cmd. */
export function typedChar(e: { key: string; ctrlKey: boolean; altKey: boolean; metaKey: boolean }): string {
  return e.key.length === 1 && e.key !== ' ' && !e.ctrlKey && !e.altKey && !e.metaKey ? e.key.toLowerCase() : ''
}

/** The text typed so far: the new character joins the buffer when it came soon after the last one. */
export function typeAhead(state: TypeAhead, char: string, now: number): TypeAhead {
  return { buffer: now - state.at <= TYPE_AHEAD_MS ? state.buffer + char : char, at: now }
}

/**
 * The row to move to, or -1 to stay. Home and End go to the ends. Typed text goes to the next row whose label starts
 * with it: after the current row when one letter is typed (pressing it again goes on to the next), from the
 * current row when more are (so "us" stays on "users" and does not skip to "usage").
 */
export function rowFor(labels: string[], current: number, key: string, typed: string): number {
  if (!labels.length) return -1
  if (key === 'Home') return 0
  if (key === 'End') return labels.length - 1
  if (!typed) return -1
  const start = typed.length > 1 ? Math.max(current, 0) : current + 1
  for (let n = 0; n < labels.length; n++) {
    const i = (start + n) % labels.length
    if (labels[i].trim().toLowerCase().startsWith(typed)) return i
  }
  return -1
}

/**
 * The item of a row's menu that a key stands for: F2 is Rename…, Delete is Delete… (which asks before it deletes).
 * Null for another key or when the row's menu has no such item.
 */
export function menuItemForKey<T extends { label: string }>(items: T[], key: string): T | null {
  const word = key === 'F2' ? 'Rename' : key === 'Delete' ? 'Delete' : ''
  return (word && items.find((i) => i.label.startsWith(word))) || null
}
