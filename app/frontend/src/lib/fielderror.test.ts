import { describe, expect, it } from 'vitest'
import { describedBy, errorId } from './fielderror'

describe('describedBy', () => {
  it('names the error message of the field', () => {
    expect(describedBy(undefined, 'Not a number', 'e1')).toBe('e1')
  })
  it('keeps the ids the field already had', () => {
    // Before: a red border was the only sign that a value was wrong.
    expect(describedBy('hint', 'Not a number', 'e1')).toBe('hint e1')
    expect(describedBy('hint', '', 'e1')).toBe('hint')
  })
  it('is undefined when there is nothing to describe', () => {
    expect(describedBy(undefined, undefined, 'e1')).toBeUndefined()
  })
})

describe('errorId', () => {
  it('gives each field its own id', () => {
    expect(errorId()).not.toBe(errorId())
  })
})
