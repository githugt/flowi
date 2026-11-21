// Save all: every open document with unsaved edits, in one key press (Ctrl+Alt+S).
// Before: each tab saved only itself, so after editing three requests the person visited three tabs.

export interface SavePlan {
  /** Tabs to save now. */
  save: string[]
  /** Drafts that have no place yet: Save asks where, so each needs its own answer. */
  needPlace: string[]
}

/** Which of the open tabs to save: the dirty ones; a draft without a place is left out and counted. */
export function planSaveAll(tabs: string[], isDirty: (node: string) => boolean, isDraft: (node: string) => boolean): SavePlan {
  const plan: SavePlan = { save: [], needPlace: [] }
  for (const node of tabs) {
    if (!isDirty(node)) continue
    ;(isDraft(node) ? plan.needPlace : plan.save).push(node)
  }
  return plan
}

/** What to tell the person after a Save all. */
export function saveAllMessage(plan: SavePlan): string {
  const n = plan.save.length
  const saved = n === 1 ? 'Saved 1 document' : `Saved ${n} documents`
  const drafts = plan.needPlace.length
  const left = drafts === 1 ? '1 draft needs a place: use Save on it' : `${drafts} drafts need a place: use Save on each`
  if (n === 0 && drafts === 0) return 'Nothing to save.'
  if (drafts === 0) return saved + '.'
  return n === 0 ? left + '.' : `${saved}. ${left}.`
}
