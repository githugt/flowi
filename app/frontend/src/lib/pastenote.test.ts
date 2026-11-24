import { describe, expect, it } from 'vitest'
import { pasteNote } from './pastenote'

describe('pasteNote', () => {
  it('says how many steps were pasted when every call was found', () => {
    expect(pasteNote(1, [])).toBe('Pasted 1 step')
    expect(pasteNote(3, [])).toBe('Pasted 3 steps')
  })
  it('names what this collection lacks and what to do', () => {
    expect(pasteNote(2, ['orders/create'])).toBe('Pasted 2 steps. This collection has no orders/create: choose another request or flow for that step.')
  })
  it('lists a few and counts the rest', () => {
    expect(pasteNote(5, ['a', 'b', 'c', 'd', 'e'])).toBe('Pasted 5 steps. This collection has no a, b, c and 2 more: choose other requests or flows for those steps.')
  })
})
