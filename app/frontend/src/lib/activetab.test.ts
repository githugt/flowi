import { describe, expect, it } from 'vitest'
import { activeToRestore, rememberable } from './activetab'
import app from './app.svelte?raw'

describe('active tab of a session', () => {
  const rows = ['a', 'b', 'c']

  it('brings back the remembered tab when it is open', () => {
    // Before: the last tab opened ended up in front, whichever one was in front at closing.
    expect(activeToRestore(rows, 'b')).toBe('b')
  })

  it('falls back to the last tab when the remembered one is gone or nothing was remembered', () => {
    expect(activeToRestore(rows, 'zzz')).toBe('c')
    expect(activeToRestore(rows, null)).toBe('c')
    expect(activeToRestore([], 'a')).toBeUndefined()
  })

  it('remembers a document tab, and keeps the last one while a screen is in front', () => {
    expect(rememberable('b', rows, 'a')).toBe('b')
    expect(rememberable('screen:settings', rows, 'a')).toBe('a')
    expect(rememberable('screen:settings', rows, null)).toBeNull()
  })

  it('opens the restored tabs quietly and chooses the front tab once', () => {
    expect(app).toContain('await this.open(n, false)')
    expect(app).toContain('activeToRestore(this.tabRows, loadActive())')
    expect(app).toContain('if (activate && current()) this.active = n.id')
  })
})
