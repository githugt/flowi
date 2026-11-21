import { describe, expect, it } from 'vitest'
import { sameData } from './same'

describe('sameData', () => {
  it('should be true for an equal copy, however deep', () => {
    // Before: an event with unchanged content replaced the state and woke every derived value that read it.
    const held = { node: 'a', steps: [{ id: 's1', ok: true }], run: null }

    expect(sameData(held, JSON.parse(JSON.stringify(held)))).toBe(true)
  })

  it('should be false when any value differs', () => {
    expect(sameData({ steps: [{ id: 's1', ok: true }] }, { steps: [{ id: 's1', ok: false }] })).toBe(false)
    expect(sameData({ a: 1 }, { a: 1, b: 2 })).toBe(false)
    expect(sameData([1, 2], [2, 1])).toBe(false)
  })

  it('should be false for nothing held yet, and true for the very same object', () => {
    const v = { a: 1 }

    expect(sameData(undefined, v)).toBe(false)
    expect(sameData(null, v)).toBe(false)
    expect(sameData(v, v)).toBe(true)
  })

  it('should count a value JSON cannot hold as changed', () => {
    const loop: Record<string, unknown> = {}
    loop.self = loop

    expect(sameData(loop, { self: 1 })).toBe(false)
  })

  it('should guard the workspace, panels, flow, environment and history events', async () => {
    const src = (await import('./app.svelte?raw')).default as string

    for (const guard of ['sameData(this.tree, data)', 'sameData(this.panels, data)', 'sameData(this.flows[v.node], v)', 'sameData(this.envs[v.node], v)', 'sameData(this.history[node], runs)']) {
      expect(src).toContain(guard)
    }
  })
})
