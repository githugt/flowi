import { describe, expect, it } from 'vitest'
import { filterRows, stepRows } from './steplist'
import type { Unit } from './types'

const unit = (id: string, nodes: string[], types: string[], parent?: string): Unit => ({ id, kind: 'card', nodes, types, path: '', x: 0, y: 0, w: 0, h: 0, parent })

describe('the step outline', () => {
  const units = [unit('u1', ['login'], ['request']), unit('u2', ['each'], ['batch']), unit('u3', ['create', 'check'], ['request', 'assert'], 'u2')]

  it('should list every step in canvas order with its depth and nothing run before a run', () => {
    // Before: a long flow was one very long row, and finding a step meant scrolling the canvas.
    const rows = stepRows(units, {}, {}, [])

    expect(rows.map((r) => [r.id, r.type, r.depth, r.status, r.mark, r.bar])).toEqual([
      ['login', 'request', 0, '', '', null],
      ['each', 'batch', 0, '', '', null],
      ['create', 'request', 1, '', '', null],
      ['check', 'assert', 1, '', '', null],
    ])
  })

  it('should show status, result line and a bar for a step that ran', () => {
    const rows = stepRows(units, { login: 'passed', create: 'failed' }, { login: '200 · 120ms' }, [
      { step: 'login', status: 'passed', startMs: 0, endMs: 100 },
      { step: 'create', status: 'failed', startMs: 100, endMs: 400 },
    ])

    expect(rows[0]).toMatchObject({ mark: '✓', result: '200 · 120ms', bar: { left: 0, width: 25 } })
    expect(rows[2]).toMatchObject({ mark: '✗', bar: { left: 25, width: 75 } })
    expect(rows[3].bar).toBeNull()
  })

  it('should not loop on a unit whose parent chain is circular', () => {
    const rows = stepRows([unit('a', ['x'], ['log'], 'b'), unit('b', ['y'], ['log'], 'a')], {}, {}, [])

    expect(rows.map((r) => r.id)).toEqual(['x', 'y'])
  })

  it('should filter by id or type, ignoring case', () => {
    const rows = stepRows(units, {}, {}, [])

    expect(filterRows(rows, 'ASSERT').map((r) => r.id)).toEqual(['check'])
    expect(filterRows(rows, 'cre').map((r) => r.id)).toEqual(['create'])
    expect(filterRows(rows, '  ')).toHaveLength(4)
  })
})
