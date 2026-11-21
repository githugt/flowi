import { describe, expect, it } from 'vitest'
import { breakHeading, updateLabel } from './specupdate'
import type { SpecUpdate } from './types'

const update = (p: Partial<SpecUpdate>): SpecUpdate => ({ spec: 'pets', added: [], changed: [], removed: [], notes: [], source: '', checked: true, breakages: [], ...p })
const broken = { request: 'pets/getPet', version: '1', where: 'a flow', reason: 'gone', reads: [] }

describe('breakHeading', () => {
  it('should say nothing when nothing was checked, and name the count or the all clear when it was', () => {
    // Before: the update preview listed files only, so it could be applied without seeing what breaks.
    expect(breakHeading(update({ checked: false }))).toBe('')
    expect(breakHeading(update({ breakages: [broken, broken, broken] }))).toBe('Would break (3)')
    expect(breakHeading(update({}))).toBe('Checked: nothing in your flows breaks')
  })
})

describe('updateLabel', () => {
  it('should say Update anyway, with the count, when it breaks something', () => {
    expect(updateLabel(update({}))).toBe('Update')
    expect(updateLabel(update({ breakages: [broken] }))).toBe('Update anyway (1 break)')
    expect(updateLabel(update({ breakages: [broken, broken] }))).toBe('Update anyway (2 breaks)')
  })
})
