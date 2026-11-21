import { describe, expect, it } from 'vitest'
import { readFold, writeFold, type FoldStorage } from './foldstate'

function memory(): FoldStorage & { data: Record<string, string> } {
  const data: Record<string, string> = {}
  return { data, getItem: (k) => data[k] ?? null, setItem: (k, v) => void (data[k] = v) }
}

describe('readFold and writeFold', () => {
  it('Given nothing kept When read Then the section is as the default says', () => {
    // Before: the inspector's General section was shut every time it was drawn.
    expect(readFold(memory(), 'when', true)).toBe(true)
    expect(readFold(memory(), 'adv', false)).toBe(false)
  })
  it('Given a choice kept When read Then it wins over the default, open or shut', () => {
    const s = memory()

    writeFold(s, 'when', false)
    writeFold(s, 'adv', true)

    expect(readFold(s, 'when', true)).toBe(false)
    expect(readFold(s, 'adv', false)).toBe(true)
  })
  it('Given a store that throws or none at all Then the default is used and nothing breaks', () => {
    const broken: FoldStorage = { getItem: () => { throw new Error('blocked') }, setItem: () => { throw new Error('blocked') } }

    expect(readFold(broken, 'x', true)).toBe(true)
    expect(() => writeFold(broken, 'x', false)).not.toThrow()
    expect(readFold(undefined, 'x', false)).toBe(false)
  })
})
