// Whether a fold-away section of a panel is open, kept per section name so a person's choice stays across steps and
// sessions. Before: the step inspector's most-used fields sat in a "General" section that was shut every time.

export interface FoldStorage {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
}

const key = (name: string) => 'fold:' + name

/** Whether the section `name` is open: what was kept, else `fallback`. A closed browser store is no reason to fail. */
export function readFold(store: FoldStorage | undefined, name: string, fallback: boolean): boolean {
  try {
    const got = store?.getItem(key(name))
    return got === '1' ? true : got === '0' ? false : fallback
  } catch {
    return fallback
  }
}

/** Keeps the choice; when the store refuses, the section just is not remembered. */
export function writeFold(store: FoldStorage | undefined, name: string, open: boolean): void {
  try {
    store?.setItem(key(name), open ? '1' : '0')
  } catch {
    // not remembered
  }
}
