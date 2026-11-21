// The right-click menu of an open tab: close it, the others, those to its right, and show where it lives.
// Before: only the sidebar rows had a context menu, so closing most tabs meant clicking each one's x.

export type CloseMode = 'others' | 'right'

/**
 * Which tabs a bulk close takes. A tab with unsaved edits is kept rather than closed, since a bulk close asks
 * nothing; `keep` lists them so the notice can say so. Home and the other screens close like any tab.
 */
export function closePlan(tabs: string[], node: string, mode: CloseMode, dirty: (id: string) => boolean): { close: string[]; keep: string[] } {
  const at = tabs.indexOf(node)
  const targets = tabs.filter((t, i) => t !== node && (mode === 'others' || (at >= 0 && i > at)))
  return { close: targets.filter((t) => !dirty(t)), keep: targets.filter((t) => dirty(t)) }
}

/** What the notice says after a bulk close: how many closed and, when some stayed, why. */
export function closeNotice(closed: number, kept: number): string {
  const tabs = (n: number) => (n === 1 ? '1 tab' : `${n} tabs`)
  if (kept === 0) return ''
  const why = `${kept === 1 ? 'has' : 'have'} unsaved changes`
  return closed > 0 ? `Closed ${tabs(closed)}. ${tabs(kept)} ${why} and stayed open.` : `${tabs(kept)} ${why} and stayed open.`
}

/** Whether a tab is a document of a collection, so its folder can be shown. Screens and hook editors are not. */
export function hasFolder(node: string): boolean {
  return !node.startsWith('screen:') && !node.startsWith('hooks:')
}
