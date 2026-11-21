import { describe, expect, it } from 'vitest'
import { valueName, valueText } from './jsonmenu'

describe('valueText', () => {
  it('Given a string Then it is copied as it is, without quotes', () => {
    // Before: a node could be picked but not copied.
    expect(valueText('hello')).toBe('hello')
  })
  it('Given a number, null or object Then it is JSON', () => {
    expect(valueText(42)).toBe('42')
    expect(valueText(null)).toBe('null')
    expect(valueText({ a: [1] })).toBe('{\n  "a": [\n    1\n  ]\n}')
  })
  it('Given a value JSON cannot show Then it still gives text', () => {
    expect(valueText(undefined)).toBe('undefined')
  })
})

describe('valueName', () => {
  it('Given a short path Then it is named, and a long one is shortened', () => {
    expect(valueName('$.a')).toBe('the value of $.a')
    expect(valueName('$.' + 'x'.repeat(60))).toMatch(/^the value at …x{30}$/)
  })
})
