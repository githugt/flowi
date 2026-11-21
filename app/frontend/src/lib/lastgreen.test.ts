import { describe, expect, it } from 'vitest'
import { lastGreenIndex } from './lastgreen'

describe('the last green run', () => {
  it('should be the newest passed run when the latest one failed', () => {
    expect(lastGreenIndex(['failed', 'failed', 'passed', 'failed', 'passed'])).toBe(2)
    expect(lastGreenIndex(['error', 'passed'])).toBe(1)
    expect(lastGreenIndex(['aborted', 'cancelled', 'passed'])).toBe(2)
  })

  it('should be nothing when the latest run passed, was cancelled, or no run ever passed', () => {
    expect(lastGreenIndex(['passed', 'failed'])).toBe(-1)
    expect(lastGreenIndex(['cancelled', 'passed'])).toBe(-1)
    expect(lastGreenIndex(['failed', 'failed'])).toBe(-1)
    expect(lastGreenIndex([])).toBe(-1)
  })
})
