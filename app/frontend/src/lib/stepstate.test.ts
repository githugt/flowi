import { describe, expect, it } from 'vitest'
import { stepMark } from './stepstate'

describe('stepMark', () => {
  it('shows a ring only for a step that is running', () => {
    expect(stepMark('running')).toBe('ring')
  })
  it('shows a dash, not a ring, for a step that was cancelled or skipped', () => {
    expect(stepMark('cancelled')).toBe('dash')
    expect(stepMark('skipped')).toBe('dash')
  })
  it('keeps the tick and the cross', () => {
    expect(stepMark('passed')).toBe('tick')
    for (const s of ['failed', 'error', 'aborted']) expect(stepMark(s)).toBe('cross')
  })
  it('shows nothing when there is no status', () => {
    expect(stepMark('')).toBe('none')
    expect(stepMark(undefined)).toBe('none')
  })
})
