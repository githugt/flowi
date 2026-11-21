import { describe, expect, it } from 'vitest'
import { closeNotice, closePlan, hasFolder } from './tabmenu'

const tabs = ['a', 'b', 'c', 'd']
const clean = () => false

describe('closePlan', () => {
  it('Given a tab When closing the others Then every other tab goes', () => {
    // Before: closing most tabs meant clicking each one's x.
    expect(closePlan(tabs, 'b', 'others', clean)).toEqual({ close: ['a', 'c', 'd'], keep: [] })
  })
  it('Given a tab When closing to the right Then only the tabs after it go', () => {
    expect(closePlan(tabs, 'b', 'right', clean).close).toEqual(['c', 'd'])
    expect(closePlan(tabs, 'd', 'right', clean).close).toEqual([])
  })
  it('Given unsaved tabs When closing in bulk Then they stay and are listed', () => {
    const got = closePlan(tabs, 'a', 'others', (id) => id === 'c')

    expect(got).toEqual({ close: ['b', 'd'], keep: ['c'] })
  })
  it('Given a tab that is not open When closing to the right Then nothing goes', () => {
    expect(closePlan(tabs, 'zzz', 'right', clean).close).toEqual([])
  })
})

describe('closeNotice', () => {
  it('Given nothing kept Then there is nothing to say', () => {
    expect(closeNotice(3, 0)).toBe('')
  })
  it('Given some kept Then it says how many closed and why the rest stayed', () => {
    expect(closeNotice(2, 1)).toBe('Closed 2 tabs. 1 tab has unsaved changes and stayed open.')
    expect(closeNotice(0, 2)).toBe('2 tabs have unsaved changes and stayed open.')
  })
})

describe('hasFolder', () => {
  it('Given a screen or hooks tab Then no folder, a document yes', () => {
    expect(hasFolder('screen:home')).toBe(false)
    expect(hasFolder('hooks:R1')).toBe(false)
    expect(hasFolder('R1')).toBe(true)
  })
})
