import { describe, expect, it } from 'vitest'
import { parseCount } from './count'

describe('parseCount', () => {
  it('reads plain digits', () => expect(parseCount('250')).toEqual({ value: 250, error: '' }))
  it('reads an empty field as 0, the default', () => expect(parseCount('  ')).toEqual({ value: 0, error: '' }))
  it('reads grouping with a comma, a dot, a space, a no-break space or an underscore', () => {
    for (const t of ['1,000', '1.000', '1 000', '1 000', '1_000']) expect(parseCount(t).value, t).toBe(1000)
  })
  it('reads full-width digits', () => expect(parseCount('１２３').value).toBe(123))
  it('does not read a decimal as grouping', () => {
    expect(parseCount('1.5').error).toBe('Write a whole number.')
    expect(parseCount('1,50').error).toBe('Write a whole number.')
  })
  it('refuses text instead of making it 0', () => {
    const got = parseCount('abc')
    expect(got.error).not.toBe('')
  })
  it('refuses a negative number', () => expect(parseCount('-5').error).toBe('Write a number of zero or more.'))
  it('refuses a number too large to be exact', () => expect(parseCount('99999999999999999999').error).toBe('That number is too large.'))
})
