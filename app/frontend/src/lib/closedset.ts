// ClosedSet remembers the documents whose tab was closed, so a late event from Go for one of them does not make
// its view again.
// Before: an autosave or run event that arrived after a tab was closed put the closed flow or request back in the
// page's state, and the next open showed a stale copy.
export class ClosedSet {
  #nodes = new Set<string>()

  /** Call when a tab closes. */
  close(...nodes: string[]) {
    for (const n of nodes) this.#nodes.add(n)
  }

  /** Call before a document is opened again: its events count from now on. */
  reopen(node: string) {
    this.#nodes.delete(node)
  }

  /** Whether an event for node should be applied. */
  accepts(node: string): boolean {
    return !this.#nodes.has(node)
  }
}
