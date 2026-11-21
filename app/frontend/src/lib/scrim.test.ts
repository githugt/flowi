import { describe, expect, it } from 'vitest'
import { fieldValues, isDirty } from './scrim'

describe('isDirty', () => {
  it('is false while the fields keep their start values', () => {
    const fields = [{ value: 'a' }, { value: '', checked: false }]
    expect(isDirty(fieldValues(fields), fields)).toBe(false)
  })
  it('is true once a field has text the dialog did not start with', () => {
    const fields = [{ value: '' }]
    const before = fieldValues(fields)
    fields[0].value = 'typed'
    expect(isDirty(before, fields)).toBe(true)
  })
  it('is true when a box is ticked or a field is added', () => {
    const fields = [{ value: 'x', checked: false }]
    const before = fieldValues(fields)
    fields[0].checked = true
    expect(isDirty(before, fields)).toBe(true)
    expect(isDirty(before, [...fields, { value: '' }])).toBe(true)
  })
})
