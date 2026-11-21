// What the empty main area teaches: the four shortcuts that start work, written for the platform.
// Before: with no tab open it said only "Open a request or a flow from the sidebar."

export interface Hint {
  label: string
  keys: string
}

/** The shortcuts shown when no tab is open. */
export function watermarkHints(mac: boolean): Hint[] {
  const mod = mac ? '⌘' : 'Ctrl+'
  const shift = mac ? '⇧' : 'Shift+'
  return [
    { label: 'New request', keys: `${mod}N` },
    { label: 'Jump to a request or flow', keys: `${mod}K` },
    { label: 'Command palette', keys: `${mod}${shift}P` },
    { label: 'Import an API', keys: `${mod}I` },
  ]
}
