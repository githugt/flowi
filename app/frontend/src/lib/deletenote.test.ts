import { describe, expect, it } from 'vitest'
import { impactNote } from './deletenote'

describe('impactNote', () => {
  it('says nothing when only the chosen step goes and nothing reads it', () => {
    expect(impactNote(1, { steps: 1, readBy: [] })).toBe('')
    expect(impactNote(1, null)).toBe('')
  })
  it('counts the steps inside a container that go too', () => {
    expect(impactNote(1, { steps: 9, readBy: [] })).toBe('8 steps inside them go too.')
    expect(impactNote(1, { steps: 2, readBy: [] })).toBe('1 step inside it goes too.')
  })
  it('names the steps that read the result', () => {
    expect(impactNote(1, { steps: 1, readBy: ['use'] })).toBe('use reads its result and will then read nothing.')
    expect(impactNote(2, { steps: 2, readBy: ['a', 'b'] })).toBe('a, b read their result and will then read nothing.')
  })
})
