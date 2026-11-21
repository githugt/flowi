import type { MatrixView } from './types'

/** What the "matrix" event carries: how many cells of the matrix running for a node have ended, of how many. */
export interface MatrixProgress {
  node: string
  done: number
  total: number
}

/**
 * The line shown while a matrix runs: the cell it is on of all of them. Before: a matrix showed only a spinner until
 * the last cell, however long that took. The cell being run is the one after the ones that ended.
 */
export function runningText(p: MatrixProgress | null): string {
  if (!p || p.total <= 0) return 'Running…'
  return `Running ${Math.min(p.done + 1, p.total)}/${p.total}`
}

/** The sentence under a result that was stopped, naming how much ran; empty for a matrix that ran to its end. */
export function stoppedNote(m: Pick<MatrixView, 'cells' | 'total' | 'cancelled'>): string {
  if (!m.cancelled) return ''
  const ran = m.cells.filter((c) => c.status !== 'cancelled').length
  return `Cancelled: ${ran} of ${m.total} ran. The rest were not run.`
}

/** True when every cell of a rows × columns grid has run, so the grid can be drawn without holes. */
export function gridComplete(m: Pick<MatrixView, 'dims' | 'cells'>): boolean {
  if (m.dims.length !== 2) return false
  return m.cells.length === m.dims[0].values.length * m.dims[1].values.length
}
