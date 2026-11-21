import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { debounced } from './debounce'

describe('debounced', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('runs once, with the last arguments, after the quiet period', () => {
    const fn = vi.fn()
    const d = debounced(fn, 500)

    d('a')
    vi.advanceTimersByTime(300)
    d('ab')
    vi.advanceTimersByTime(300)
    expect(fn).not.toHaveBeenCalled()
    vi.advanceTimersByTime(300)

    expect(fn).toHaveBeenCalledTimes(1)
    expect(fn).toHaveBeenCalledWith('ab')
  })
  it('flush runs the waiting call now, and only once', () => {
    const fn = vi.fn()
    const d = debounced(fn, 500)
    d('x')

    d.flush()
    d.flush()
    vi.advanceTimersByTime(1000)

    expect(fn).toHaveBeenCalledTimes(1)
  })
  it('cancel drops the waiting call', () => {
    const fn = vi.fn()
    const d = debounced(fn, 500)
    d('x')

    d.cancel()
    vi.advanceTimersByTime(1000)

    expect(fn).not.toHaveBeenCalled()
  })
})
