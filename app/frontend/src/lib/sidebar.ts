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

/**
 * The rule for folding: `auto` folds for a flow (the default), `open` never folds by itself, `manual` leaves the
 * sidebar as the person set it.
 * Before: the sidebar folded when a flow opened, with no setting, no key and no word about why.
 */
export type SidebarMode = 'auto' | 'open' | 'manual'

export const SIDEBAR_MODES: { id: SidebarMode; title: string }[] = [
  { id: 'auto', title: 'Sidebar: fold when a flow opens' },
  { id: 'open', title: 'Sidebar: always open' },
  { id: 'manual', title: 'Sidebar: only fold by hand' },
]

/** Whether the sidebar is folded after a tab of `kind` came forward, given whether it was and what is pinned. */
export function collapsedOnOpen(kind: TabKind, collapsed: boolean, pins: Pins, mode: SidebarMode = 'auto'): boolean {
  if (mode === 'open') return false
  if (mode === 'manual') return collapsed
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

const MODE_KEY = 'sidebar:mode'
const TOLD_KEY = 'sidebar:told'

export function loadMode(s: Storage): SidebarMode {
  try {
    const v = s.getItem(MODE_KEY)
    return v === 'open' || v === 'manual' ? v : 'auto'
  } catch {
    return 'auto'
  }
}

export function saveMode(s: Storage, mode: SidebarMode) {
  try {
    s.setItem(MODE_KEY, mode)
  } catch {
    // the choice just is not remembered
  }
}

/** True the first time the sidebar folds by itself: that is when to say why. Marks it as told. */
export function shouldExplainFold(s: Storage): boolean {
  try {
    if (s.getItem(TOLD_KEY)) return false
    s.setItem(TOLD_KEY, '1')
    return true
  } catch {
    return false // no storage: better silent than a toast at every fold
  }
}

export const FOLD_NOTE = 'The sidebar folds when a flow opens, to give the graph room. Press Ctrl+B to bring it back, or choose "Sidebar: always open" in the command palette.'
