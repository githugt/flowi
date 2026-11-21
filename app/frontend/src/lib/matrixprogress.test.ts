import { describe, expect, it } from 'vitest'
import { gridComplete, runningText, stoppedNote } from './matrixprogress'

const cell = (status: 'passed' | 'cancelled') => ({ labels: ['x'], status })
const dims = [{ name: 'env', values: ['a', 'b'] }, { name: 'overlay', values: ['x', 'y', 'z'] }]

describe('runningText', () => {
  it('should name the cell being run out of all of them', () => {
    // Before: only a spinner showed until the last cell.
    expect(runningText({ node: 'n', done: 6, total: 12 })).toBe('Running 7/12')
    expect(runningText({ node: 'n', done: 0, total: 12 })).toBe('Running 1/12')
  })

  it('should not go past the total, and say plain Running before the first report', () => {
    expect(runningText({ node: 'n', done: 12, total: 12 })).toBe('Running 12/12')
    expect(runningText(null)).toBe('Running…')
  })
})

describe('stoppedNote', () => {
  it('should be empty for a matrix that ran to its end', () => {
    expect(stoppedNote({ cells: [cell('passed')], total: 1, cancelled: false })).toBe('')
  })

  it('should count the cells that ran, not the one the stop cut short', () => {
    const note = stoppedNote({ cells: [cell('passed'), cell('passed'), cell('cancelled')], total: 12, cancelled: true })

    expect(note).toBe('Cancelled: 2 of 12 ran. The rest were not run.')
  })
})

describe('gridComplete', () => {
  it('should be true only when every cell of two dimensions is there', () => {
    expect(gridComplete({ dims, cells: Array(6).fill(cell('passed')) })).toBe(true)
    expect(gridComplete({ dims, cells: Array(4).fill(cell('passed')) })).toBe(false)
    expect(gridComplete({ dims: dims.slice(0, 1), cells: Array(2).fill(cell('passed')) })).toBe(false)
  })
})
