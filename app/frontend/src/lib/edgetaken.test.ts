import { describe, expect, it } from 'vitest'
import { edgeTaken, running, throughStatus } from './edgetaken'

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
  // Before: an arrow out of a skipped step was never lit, so a run that went on through skipped steps looked cut in pieces.
  it('takes the arrow out of a skipped step when the step after it ran', () => {
    expect(edgeTaken({}, 'skipped', 'passed')).toBe(true)
  })
  it('does not take the arrow out of a skipped step when the step after it did not run, or its failure arrow', () => {
    expect(edgeTaken({}, 'skipped', 'skipped')).toBe(false)
    expect(edgeTaken({}, 'skipped', '')).toBe(false)
    expect(edgeTaken({ fail: true }, 'skipped', 'passed')).toBe(false)
  })
})

describe('throughStatus', () => {
  it('counts a step skipped on purpose as passed, for its arrows', () => {
    expect(throughStatus('skipped', true)).toBe('passed')
  })
  it('leaves a step skipped by its when, or any other status, as it is', () => {
    expect(throughStatus('skipped', false)).toBe('skipped')
    expect(throughStatus('failed', true)).toBe('failed')
    expect(throughStatus('passed', true)).toBe('passed')
  })
  it('lights the whole path through skipped steps: passed, skipped, skipped, passed', () => {
    const [a, b, c, d] = ['passed', 'skipped', 'skipped', 'passed'].map((s, i) => throughStatus(s, i === 1 || i === 2))

    expect([edgeTaken({}, a, b), edgeTaken({}, b, c), edgeTaken({}, c, d)]).toEqual([true, true, true])
  })
})

describe('running', () => {
  it('is true only while a step runs', () => {
    expect(running({ a: 'passed', b: 'running' })).toBe(true)
    expect(running({ a: 'passed' })).toBe(false)
    expect(running({})).toBe(false)
  })
})
