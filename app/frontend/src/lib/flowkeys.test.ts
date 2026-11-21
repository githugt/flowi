import { describe, expect, it } from 'vitest'
import { clipAction } from './flowkeys'

const press = (letter: string) => ({ key: letter, code: 'Key' + letter.toUpperCase() })
const ctx = { typing: false, editing: true, selected: 2, textSelected: false }

describe('clipAction', () => {
  it('Given Edit on When A, C or V is pressed Then it selects all, copies and pastes', () => {
    // Before: the canvas answered only Undo, Save and Delete.
    expect(clipAction(press('a'), ctx)).toBe('selectAll')
    expect(clipAction(press('c'), ctx)).toBe('copy')
    expect(clipAction(press('v'), ctx)).toBe('paste')
  })
  it('Given focus in a field When pressed Then the field keeps its own select, copy and paste', () => {
    for (const l of ['a', 'c', 'v']) expect(clipAction(press(l), { ...ctx, typing: true })).toBeNull()
  })
  it('Given Edit off When pressed Then selecting all and pasting do nothing but copy still works', () => {
    const view = { ...ctx, editing: false }

    expect(clipAction(press('a'), view)).toBeNull()
    expect(clipAction(press('v'), view)).toBeNull()
    expect(clipAction(press('c'), view)).toBe('copy')
  })
  it('Given no steps selected or text selected When C is pressed Then the page copies as it did', () => {
    expect(clipAction(press('c'), { ...ctx, selected: 0 })).toBeNull()
    expect(clipAction(press('c'), { ...ctx, textSelected: true })).toBeNull()
  })
  it('Given a Cyrillic layout When the same keys are pressed Then they still act', () => {
    expect(clipAction({ key: 'ф', code: 'KeyA' }, ctx)).toBe('selectAll')
  })
  it('Given another letter When pressed Then nothing', () => {
    expect(clipAction(press('x'), ctx)).toBeNull()
  })
})
