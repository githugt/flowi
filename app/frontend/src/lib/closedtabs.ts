// The tabs that were closed, newest first, so Ctrl+Shift+T brings the last one back.
// Before: a tab closed by a slip was gone until it was found again in the tree.

export interface ClosedTab {
  node: string
  kind: string
}

export const REOPEN_LIMIT = 10

/** Only documents come back: a screen is one keystroke away anyway and a hooks editor belongs to an open request. */
const DOCUMENTS = ['request', 'flow', 'environment']

/** The list after `tab` was closed: newest first, one entry per document, at most REOPEN_LIMIT. */
export function rememberClosed(list: ClosedTab[], tab: ClosedTab): ClosedTab[] {
  if (!DOCUMENTS.includes(tab.kind)) return list
  return [{ node: tab.node, kind: tab.kind }, ...list.filter((t) => t.node !== tab.node)].slice(0, REOPEN_LIMIT)
}

/**
 * The tab to bring back and the list without it. A document that is open again, or that no longer exists
 * (`usable` says no), is dropped on the way. Null when nothing is left.
 */
export function takeReopen(list: ClosedTab[], usable: (node: string) => boolean): { tab: ClosedTab; rest: ClosedTab[] } | null {
  for (let i = 0; i < list.length; i++) {
    if (usable(list[i].node)) return { tab: list[i], rest: list.slice(i + 1) }
  }
  return null
}
