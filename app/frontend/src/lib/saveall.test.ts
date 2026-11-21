import { describe, expect, it } from 'vitest'
import { planSaveAll, saveAllMessage } from './saveall'
import { windowKeyAction } from './shortcuts'

describe('save all', () => {
  const dirty = new Set(['a', 'c', 'draft'])
  const plan = planSaveAll(['a', 'b', 'c', 'draft'], (n) => dirty.has(n), (n) => n === 'draft')

  it('saves the dirty tabs and leaves a draft for its own Save', () => {
    // Before: each tab saved only itself.
    expect(plan.save).toEqual(['a', 'c'])
    expect(plan.needPlace).toEqual(['draft'])
  })

  it('says what happened', () => {
    expect(saveAllMessage(plan)).toBe('Saved 2 documents. 1 draft needs a place: use Save on it.')
    expect(saveAllMessage({ save: ['a'], needPlace: [] })).toBe('Saved 1 document.')
    expect(saveAllMessage({ save: [], needPlace: [] })).toBe('Nothing to save.')
    expect(saveAllMessage({ save: [], needPlace: ['x', 'y'] })).toBe('2 drafts need a place: use Save on each.')
  })

  it('is on Ctrl+Alt+S, found by key position on a Mac', () => {
    const e = (o: object) => ({ key: 's', code: 'KeyS', ctrlKey: false, metaKey: false, altKey: false, shiftKey: false, ...o })

    expect(windowKeyAction(e({ ctrlKey: true, altKey: true }), false)).toBe('save-all')
    expect(windowKeyAction(e({ metaKey: true, altKey: true, key: 'ß' }), true)).toBe('save-all')
    expect(windowKeyAction(e({ ctrlKey: true }), false)).toBe('')
  })
})
