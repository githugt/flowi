// A file dropped on the window must not take the webview to that file. The page has no drop target of its own,
// so the browser's default (open the file here, leaving the app) is the only thing a stray drop can do.
// Before: dropping a file on the window replaced the whole app with the file's contents.

interface DropLike {
  type: string
  dataTransfer?: { types: readonly string[] } | null
  target: unknown
  preventDefault(): void
}

/** Whether a drag carries files (text dragged inside a field is the browser's own business). */
export function carriesFiles(e: Pick<DropLike, 'dataTransfer'>): boolean {
  return !!e.dataTransfer && Array.from(e.dataTransfer.types).includes('Files')
}

/** Cancels the default of a drag or drop of files; a file input keeps its own handling. */
export function guardDrop(e: DropLike): boolean {
  if (!carriesFiles(e)) return false
  const t = e.target as { tagName?: string; type?: string } | null
  if (t?.tagName === 'INPUT' && t.type === 'file') return false
  e.preventDefault()
  return true
}

/** Installs the guard on a window; returns what removes it. */
export function installDropGuard(win: Pick<Window, 'addEventListener' | 'removeEventListener'>): () => void {
  const on = (e: Event) => void guardDrop(e as unknown as DropLike)
  win.addEventListener('dragover', on)
  win.addEventListener('drop', on)
  return () => {
    win.removeEventListener('dragover', on)
    win.removeEventListener('drop', on)
  }
}
