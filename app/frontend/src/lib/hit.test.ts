import { describe, expect, it } from 'vitest'
import { innermost } from './hit'

describe('innermost', () => {
  const loop = { id: 'loop', x: 0, y: 0, w: 400, h: 200 }
  const step = { id: 'step', x: 50, y: 50, w: 100, h: 60 }

  it('prefers the step in a lane over its container, whatever the list order', () => {
    expect(innermost([loop, step], 70, 70)?.id).toBe('step')
    expect(innermost([step, loop], 70, 70)?.id).toBe('step')
  })

  it('falls back to the container outside the step, and to nothing outside both', () => {
    expect(innermost([loop, step], 300, 150)?.id).toBe('loop')
    expect(innermost([loop, step], 500, 500)).toBeUndefined()
  })
})
