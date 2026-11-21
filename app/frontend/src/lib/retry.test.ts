import { describe, expect, it } from 'vitest'
import { firstFailed } from './retry'

describe('firstFailed', () => {
  it('returns the failed step that comes first in the flow, not in the status map', () => {
    expect(firstFailed({ c: 'failed', a: 'passed', b: 'error' }, ['a', 'b', 'c'])).toBe('b')
  })
  it('returns nothing when no step failed', () => {
    expect(firstFailed({ a: 'passed', b: 'skipped' }, ['a', 'b'])).toBe('')
    expect(firstFailed(undefined, ['a'])).toBe('')
  })
  it('still finds a failed step that the canvas does not list', () => {
    expect(firstFailed({ z: 'failed', y: 'failed' }, ['a'])).toBe('y')
  })
})
