import { describe, expect, it } from 'vitest'
import { MAX_MATRIX_CELLS, matrixCells, matrixNote } from './matrixlimit'

const dim = (n: number) => ({ values: Array.from({ length: n }, (_, i) => i) })

describe('matrixCells', () => {
  it('should multiply the value counts, and be one for none', () => {
    expect(matrixCells([dim(3), dim(4)])).toBe(12)
    expect(matrixCells([])).toBe(1)
  })
})

describe('matrixNote', () => {
  it('should say nothing before anything is picked', () => {
    expect(matrixNote([], 'send')).toEqual({ text: '', over: false })
  })

  it('should give the count in the right number while it fits, up to the limit', () => {
    expect(matrixNote([dim(1)], 'send')).toEqual({ text: '1 send', over: false })
    expect(matrixNote([dim(10), dim(10)], 'run')).toEqual({ text: '100 runs', over: false })
  })

  it('should say why Run is off and what to do when there are too many', () => {
    // Before: Run went grey above 100 and only the count stood beside it.
    const note = matrixNote([dim(11), dim(10)], 'send')

    expect(note.over).toBe(true)
    expect(note.text).toBe('110 sends: the most one run does is 100. Pick fewer values.')
  })

  it('should use the same limit as the Go side', async () => {
    // @ts-ignore
    const { readFileSync } = (await import('node:fs')) as { readFileSync: (url: URL, enc: string) => string }
    const go = readFileSync(new URL('../../../internal/vm/matrix.go', import.meta.url), 'utf8')

    expect(go).toContain(`MaxMatrixCells = ${MAX_MATRIX_CELLS}`)
  })

  it('should be what both matrix dialogs show and gate Run with', async () => {
    const request = (await import('../RequestMatrix.svelte?raw')).default as string
    const flow = (await import('../FlowTools.svelte?raw')).default as string

    for (const src of [request, flow]) {
      expect(src).toContain('matrixNote(dims,')
      expect(src).toContain('note.over')
      expect(src).not.toContain('cellCount > 100')
    }
  })
})
