// Editing a name where it is shown: Enter or leaving the field keeps it, Escape puts the old one back.
// Before: the step id field kept a new name only on Enter, so clicking away lost it without a word.

export type InlineAction = 'commit' | 'revert' | ''

/** What a key does to a name being edited. */
export function inlineKey(key: string, composing = false): InlineAction {
  if (composing) return '' // a key that picks a character in an input method is not a command
  if (key === 'Enter') return 'commit'
  if (key === 'Escape') return 'revert'
  return ''
}

/** The name to keep, or null when there is nothing to keep: empty, or the same as before. */
export function nameToKeep(draft: string, current: string): string | null {
  const next = draft.trim()
  return next === '' || next === current ? null : next
}

/** The text the field shows: what is being edited, or the name itself once editing is over. */
export function shownName(draft: string, current: string, editing: boolean): string {
  return editing ? draft : current
}
