// Which tab was in front when the window closed, kept for the next start. The open tabs are stored by Go; which one is
// active is a per-window convenience, kept here.
// Before: a restored session opened every tab in turn and left the last one in front, and the active tab flipped once
// for each tab on the way.

const KEY = 'flowi.session.active'

/** The tab to bring to the front after the tabs are back: the remembered one when it is open, else the last (the old behaviour). */
export function activeToRestore(rows: string[], remembered: string | null): string | undefined {
  if (remembered && rows.includes(remembered)) return remembered
  return rows[rows.length - 1]
}

/** What to remember when `active` is in front: a document tab is remembered; a screen (Settings, Home) leaves the last one as it was. */
export function rememberable(active: string, rows: string[], previous: string | null): string | null {
  return rows.includes(active) ? active : previous
}

export function loadActive(): string | null {
  try {
    return localStorage.getItem(KEY)
  } catch {
    return null
  }
}

export function saveActive(row: string | null) {
  try {
    if (row) localStorage.setItem(KEY, row)
    else localStorage.removeItem(KEY)
  } catch {
    // storage may be blocked: the last tab then comes to the front, as it always did
  }
}
