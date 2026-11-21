import { describe, expect, it } from 'vitest'
import { acceptsText, defaultNote } from './settinghint'

describe('acceptsText', () => {
  it('gives the range of a bounded number and an example for a duration', () => {
    expect(acceptsText({ kind: 'int', min: 1, max: 100 })).toBe('A whole number from 1 to 100.')
    expect(acceptsText({ kind: 'int', min: 1 })).toBe('A whole number, 1 or more.')
    expect(acceptsText({ kind: 'int' })).toBe('A whole number.')
    expect(acceptsText({ kind: 'duration' })).toContain('30s')
  })
  it('says nothing for a kind whose description is enough', () => {
    expect(acceptsText({ kind: 'string' })).toBe('')
    expect(acceptsText({ kind: 'bool' })).toBe('')
  })
})

describe('defaultNote', () => {
  it('names the default once the value is changed', () => {
    expect(defaultNote({ userSet: true, value: '10s', default: '30s' })).toBe('Default: 30s')
  })
  it('stays quiet when the value is the default, or is not set, or the default is unknown', () => {
    expect(defaultNote({ userSet: true, value: '30s', default: '30s' })).toBe('')
    expect(defaultNote({ userSet: false, value: '30s', default: '30s' })).toBe('')
    expect(defaultNote({ userSet: true, value: '10s' })).toBe('')
  })
})
