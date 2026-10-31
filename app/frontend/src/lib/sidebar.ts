// When the sidebar folds to its rail. A flow wants the whole window for its graph, so a flow tab folds the sidebar
// unless it is pinned open for flows; a request tab leaves it as it was; a pin overrides. The pin is kept for each
// kind of tab in the browser's storage, as the pane widths are.

export type TabKind = 'flow' | 'request' | 'other'
export interface Pins {
  flow: boolean
  request: boolean
}

interface Storage {
  getItem(k: string): string | null
  setItem(k: string, v: string): void
}

const KEY = 'sidebar:pins'

/** Whether the sidebar is folded after a tab of `kind` came forward, given whether it was and what is pinned. */
export function collapsedOnOpen(kind: TabKind, collapsed: boolean, pins: Pins): boolean {
  if (kind === 'flow') return !pins.flow
  if (kind === 'request' && pins.request) return false
  return collapsed
}

export function loadPins(s: Storage): Pins {
  try {
    const v = JSON.parse(s.getItem(KEY) ?? 'null')
    return { flow: v?.flow === true, request: v?.request === true }
  } catch {
    return { flow: false, request: false }
  }
}

export function savePins(s: Storage, pins: Pins) {
  try {
    s.setItem(KEY, JSON.stringify(pins))
  } catch {
    // the choice just is not remembered
  }
}
