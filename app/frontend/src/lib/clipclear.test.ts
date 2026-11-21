import { describe, expect, it } from 'vitest'
import { CLEAR_AFTER_MS, copyAndClear, type ClipboardPort } from './clipclear'
import { revealsNote } from './revealsnote'
import screen from '../SecretsScreen.svelte?raw'

function fakeClipboard() {
  const state = { text: '', pending: undefined as undefined | (() => void), ms: 0, cancelled: false }
  const port: ClipboardPort = {
    write: async (t) => void (state.text = t),
    read: async () => state.text,
    later: (fn, ms) => {
      state.pending = fn
      state.ms = ms
      return () => void (state.cancelled = true)
    },
  }
  return { state, port }
}

describe('copyAndClear', () => {
  it('copies, then clears after 30 seconds while the clipboard still holds the secret', async () => {
    // Before: a secret could not be copied, or it stayed on the clipboard for good.
    const { state, port } = fakeClipboard()
    await copyAndClear('hunter2', port)

    expect(state.text).toBe('hunter2')
    expect(state.ms).toBe(CLEAR_AFTER_MS)
    await (state.pending as unknown as () => Promise<void>)()
    expect(state.text).toBe('')
  })

  it('leaves the clipboard alone when something else was copied since', async () => {
    const { state, port } = fakeClipboard()
    await copyAndClear('hunter2', port)
    state.text = 'something else'

    await (state.pending as unknown as () => Promise<void>)()

    expect(state.text).toBe('something else')
  })

  it('leaves it when it cannot be read, and returns what cancels the timer', async () => {
    const { state, port } = fakeClipboard()
    port.read = async () => {
      throw new Error('no clipboard')
    }
    const cancel = await copyAndClear('x', port)
    await (state.pending as unknown as () => Promise<void>)()

    expect(state.text).toBe('x')
    cancel()
    expect(state.cancelled).toBe(true)
  })
})

describe('secrets screen', () => {
  it('counts reveals and offers Copy on a shown value', () => {
    expect(revealsNote(1)).toBe('Revealed 1 time this session.')
    expect(revealsNote(3)).toBe('Revealed 3 times this session.')
    expect(screen).toContain('revealsNote(view.reveals)')
    expect(screen).toContain('copyAndClear(')
  })
})
