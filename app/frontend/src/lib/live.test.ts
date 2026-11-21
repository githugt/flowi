import { beforeEach, describe, expect, it, vi } from 'vitest'

const handlers: Record<string, (e: { data: unknown }) => void> = {}
vi.mock('@wailsio/runtime', () => ({
  Events: {
    On: (name: string, fn: (e: { data: unknown }) => void) => {
      handlers[name] = fn
      return () => delete handlers[name]
    },
  },
}))

import { live } from './live'

describe('live', () => {
  beforeEach(() => {
    for (const k of Object.keys(handlers)) delete handlers[k]
  })

  it('sets the state once, and then follows the event', async () => {
    const set = vi.fn()

    live('settings', () => Promise.resolve('first'), set)
    await Promise.resolve()
    handlers['settings']({ data: 'next' })

    expect(set.mock.calls).toEqual([['first'], ['next']])
  })
  it('drops a state that arrives after a newer event', async () => {
    const set = vi.fn()
    let answer: (v: unknown) => void = () => {}

    live('settings', () => new Promise((r) => (answer = r)), set)
    handlers['settings']({ data: 'event' })
    answer('late')
    await Promise.resolve()

    expect(set.mock.calls).toEqual([['event']])
  })
  it('hands a failed state call to fail, which it dropped before', async () => {
    const fail = vi.fn()

    // Before: .catch(() => {}) swallowed it, so a failed SettingsService.State() left no message.
    live('settings', () => Promise.reject(new Error('disk')), () => {}, fail)
    await new Promise((r) => setTimeout(r, 0))

    expect(fail).toHaveBeenCalledTimes(1)
    expect((fail.mock.calls[0][0] as Error).message).toBe('disk')
  })
  it('stops following when the returned function is called', () => {
    const off = live('settings', () => new Promise(() => {}), () => {})
    off()
    expect(handlers['settings']).toBeUndefined()
  })
})
