import { describe, expect, it } from 'vitest'
import { noun, plural } from './plural'

describe('plural', () => {
  it('uses the singular for one only', () => {
    expect(plural(1, 'item')).toBe('1 item')
    expect(plural(0, 'item')).toBe('0 items')
    expect(plural(2, 'item')).toBe('2 items')
  })
  it('takes an irregular plural', () => {
    expect(plural(1, 'key')).toBe('1 key')
    expect(plural(3, 'entry', 'entries')).toBe('3 entries')
  })
  it('gives the noun alone', () => {
    expect(noun(1, 'tab')).toBe('tab')
    expect(noun(5, 'tab')).toBe('tabs')
  })
})
