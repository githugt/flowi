import { describe, expect, it } from 'vitest'
import { guardDrop, installDropGuard } from './dropguard'

const drag = (types: string[], target: unknown = {}) => {
  const e = { type: 'drop', dataTransfer: { types }, target, prevented: false, preventDefault() { this.prevented = true } }
  return e
}

describe('guardDrop', () => {
  it('cancels a drop of files on the page', () => {
    // Before: the webview opened the dropped file and the app was gone.
    const e = drag(['Files'])
    expect(guardDrop(e)).toBe(true)
    expect(e.prevented).toBe(true)
  })
  it('leaves text dragged inside a field alone', () => {
    const e = drag(['text/plain'])
    expect(guardDrop(e)).toBe(false)
    expect(e.prevented).toBe(false)
  })
  it('leaves a file input to handle its own drop', () => {
    const e = drag(['Files'], { tagName: 'INPUT', type: 'file' })
    expect(guardDrop(e)).toBe(false)
    expect(e.prevented).toBe(false)
  })
  it('does nothing when the event carries no data', () => {
    expect(guardDrop({ type: 'drop', dataTransfer: null, target: null, preventDefault() {} })).toBe(false)
  })
})

describe('installDropGuard', () => {
  it('listens for dragover and drop, and removes both again', () => {
    const on = new Set<string>()
    const win = { addEventListener: (t: string) => void on.add(t), removeEventListener: (t: string) => void on.delete(t) }
    const off = installDropGuard(win as unknown as Window)
    expect([...on].sort()).toEqual(['dragover', 'drop'])
    off()
    expect(on.size).toBe(0)
  })
})
