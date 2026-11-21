import { describe, expect, it } from 'vitest'
import { crashDetails } from './crashtext'

describe('crashDetails', () => {
  it('names the tab and the error, and keeps the stack short', () => {
    const e = new TypeError('x is undefined')
    e.stack = Array.from({ length: 30 }, (_, i) => `at f${i}`).join('\n')
    const t = crashDetails(e, 'GET /pets')
    expect(t).toContain('"GET /pets"')
    expect(t).toContain('TypeError: x is undefined')
    expect(t.split('\n').length).toBeLessThanOrEqual(10)
  })

  it('accepts a thrown string', () => {
    expect(crashDetails('boom', 't')).toContain('Error: boom')
  })
})
