import { plural } from './plural'

/** The most cells one matrix run does. The same number as vm.MaxMatrixCells in Go (a test keeps the two equal). */
export const MAX_MATRIX_CELLS = 100

/** How many runs the chosen dimensions make: the product of their value counts. */
export function matrixCells(dims: { values: unknown[] }[]): number {
  return dims.reduce((n, d) => n * d.values.length, 1)
}

/**
 * The line under the matrix choices: nothing before anything is picked, the count when it fits, and when it is too
 * many, why Run is off and what to do. Before: Run went grey above 100 with only the count beside it, so nothing said
 * why.
 */
export function matrixNote(dims: { values: unknown[] }[], unit: string): { text: string; over: boolean } {
  if (!dims.length) return { text: '', over: false }
  const cells = matrixCells(dims)
  if (cells <= MAX_MATRIX_CELLS) return { text: plural(cells, unit), over: false }
  return { text: `${plural(cells, unit)}: the most one run does is ${MAX_MATRIX_CELLS}. Pick fewer values.`, over: true }
}
