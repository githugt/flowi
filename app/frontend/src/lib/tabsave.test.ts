import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createTabSaver } from './tabsave'

describe('createTabSaver', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('saves once after a burst of changes, with the last list', () => {
    const save = vi.fn()
    const saver = createTabSaver(save, 300)

    // Before: each of these was a call to Go.
    saver(['a'])
    saver(['a', 'b'])
    saver(['b', 'a'])
    vi.advanceTimersByTime(299)
    expect(save).not.toHaveBeenCalled()
    vi.advanceTimersByTime(2)

    expect(save).toHaveBeenCalledTimes(1)
    expect(save).toHaveBeenCalledWith(['b', 'a'])
  })
  it('does not save a list equal to the one saved before', () => {
    const save = vi.fn()
    const saver = createTabSaver(save, 100)
    saver(['a', 'b'])
    vi.advanceTimersByTime(100)

    saver(['a', 'b']) // the active tab changed: a new array, the same ids
    vi.advanceTimersByTime(100)

    expect(save).toHaveBeenCalledTimes(1)
  })
  it('saves a changed list again, and flush saves at once', () => {
    const save = vi.fn()
    const saver = createTabSaver(save, 100)
    saver(['a'])
    vi.advanceTimersByTime(100)

    saver([])
    saver.flush()

    expect(save).toHaveBeenLastCalledWith([])
    expect(save).toHaveBeenCalledTimes(2)
  })
})
