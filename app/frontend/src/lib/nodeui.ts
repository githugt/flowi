// The small UI state of an open editor that should survive switching to another tab and back: the section chosen,
// the hook mode of the next send, the replay bar. A tab switch remounts the editor, so the state is kept here by node.
// Before: each of these went back to its first value on every switch, and "Run no hooks" was silently forgotten.

export interface NodeUi {
  /** The request section chosen ('' for the first). */
  section: string
  /** The hook mode of the next send: all, none, skip-pre or skip-post. */
  hooks: string
  /** The replay bar is open. */
  replay: boolean
}

const state = new Map<string, NodeUi>()
const fresh = (): NodeUi => ({ section: '', hooks: 'all', replay: false })

/** The state of a node: a copy, so a caller changes it only through setNodeUi. */
export function nodeUi(node: string): NodeUi {
  return { ...(state.get(node) ?? fresh()) }
}

/** Changes some of a node's state. */
export function setNodeUi(node: string, patch: Partial<NodeUi>) {
  state.set(node, { ...(state.get(node) ?? fresh()), ...patch })
}

/** Forgets a node, when its tab is closed or its file goes. */
export function forgetNodeUi(node: string) {
  state.delete(node)
}
