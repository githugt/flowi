import { describe, expect, it } from 'vitest'
import { statusMark } from './statusmark'

describe('statusMark', () => {
  it('gives each known status its own shape', () => {
    const shapes = ['passed', 'failed', 'error', 'aborted', 'running', 'skipped'].map(statusMark)

    expect(new Set(shapes).size).toBe(shapes.length)
  })
  it('gives a question mark to a status it does not know', () => expect(statusMark('weird')).toBe('?'))
})
