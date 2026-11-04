// What the window remembers about how its editor is divided, per workspace: the layout it was left in, and the
// layouts the person named. Kept in the web view's own storage, which can be unavailable (a private window) or hold
// anything: every read and write is guarded, and the layout is then simply not remembered.

const lastKey = (workspace: string) => `flowi.layout.${workspace}`
const namedKey = (workspace: string) => `flowi.layouts.${workspace}`

function read(key: string): unknown {
  try {
    return JSON.parse(localStorage.getItem(key) ?? 'null')
  } catch {
    return null
  }
}

function write(key: string, value: unknown | null) {
  try {
    if (value === null) localStorage.removeItem(key)
    else localStorage.setItem(key, JSON.stringify(value))
  } catch {
    // not kept: the next window starts with one pane
  }
}

/** The layout the window was left in (as encodeLayout wrote it), null when there was none. */
export const readLast = (workspace: string): unknown => read(lastKey(workspace))

/** Remembers the layout the window is in; null forgets it (one pane needs no remembering). */
export const writeLast = (workspace: string, encoded: unknown | null) => write(lastKey(workspace), encoded)

/** The layouts named by the person, by name. Anything that is not a name with a layout is left out. */
export function readNamed(workspace: string): Record<string, unknown> {
  const data = read(namedKey(workspace))
  if (!data || typeof data !== 'object' || Array.isArray(data)) return {}
  return Object.fromEntries(Object.entries(data as Record<string, unknown>).filter(([name, layout]) => name && layout && typeof layout === 'object'))
}

export const writeNamed = (workspace: string, named: Record<string, unknown>) => write(namedKey(workspace), Object.keys(named).length ? named : null)
