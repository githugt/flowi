import { describe, expect, it } from 'vitest'
import { edgeTaken, running } from './edgetaken'

describe('edgeTaken', () => {
  it('takes an arrow from a passed step to one that ran', () => {
    expect(edgeTaken({}, 'passed', 'passed')).toBe(true)
  })
  it('does not take an arrow to a step that was skipped or never ran', () => {
    expect(edgeTaken({}, 'passed', 'skipped')).toBe(false)
    expect(edgeTaken({}, 'passed', '')).toBe(false)
  })
  it('does not take an arrow from a failed step', () => {
    expect(edgeTaken({}, 'failed', 'passed')).toBe(false)
  })
  it('takes a failure arrow only after a failure, and only when its target ran', () => {
    expect(edgeTaken({ fail: true }, 'failed', 'passed')).toBe(true)
    expect(edgeTaken({ fail: true }, 'passed', 'passed')).toBe(false)
    expect(edgeTaken({ fail: true }, 'failed', 'skipped')).toBe(false)
  })
  it('shows a run reaching forward from a step that is still running', () => {
    expect(edgeTaken({}, 'running', '')).toBe(true)
  })
})

describe('running', () => {
  it('is true only while a step runs', () => {
    expect(running({ a: 'passed', b: 'running' })).toBe(true)
    expect(running({ a: 'passed' })).toBe(false)
    expect(running({})).toBe(false)
  })
})
