import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { settle } from './settle'

describe('settle', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('returns true as soon as the state is done', async () => {
    let n = 0
    const p = settle(() => ++n >= 3, 1000, 10)
    await vi.advanceTimersByTimeAsync(50)

    expect(await p).toBe(true)
  })
  it('returns false when the state never gets done in time', async () => {
    const p = settle(() => false, 100, 10)
    await vi.advanceTimersByTimeAsync(200)

    expect(await p).toBe(false)
  })
})
