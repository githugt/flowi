import { describe, expect, it } from 'vitest'
import { scheduleStatus } from './schedulerow'

const now = Date.parse('2026-01-01T00:00:00Z')
const row = (over = {}) => ({ disabled: false, flowLabel: 'Shop › checkout', nextAt: '2026-01-01T03:00:00Z', ...over })

describe('scheduleStatus', () => {
  // Before: every row with a nextAt said "Next run in …", whether or not the runner was on or the flow existed.
  it('counts down when the runner is on and the flow exists', () => {
    expect(scheduleStatus(row(), true, now)).toEqual({ text: 'Next run in 3 h', firing: true })
  })
  it('says the schedules are paused when the runner is off', () => {
    expect(scheduleStatus(row(), false, now)).toEqual({ text: 'Paused: the schedules are not firing', firing: false })
  })
  it('says the flow is missing instead of counting down', () => {
    expect(scheduleStatus(row({ flowLabel: '' }), true, now).text).toMatch(/Flow not found/)
  })
  it('keeps Disabled first, and Never fires when no time exists', () => {
    expect(scheduleStatus(row({ disabled: true, flowLabel: '' }), false, now).text).toBe('Disabled')
    expect(scheduleStatus(row({ nextAt: '' }), true, now).text).toBe('Never fires')
  })
  it('says Due now once the time has passed, not "Next run " with nothing after it', () => {
    expect(scheduleStatus(row({ nextAt: '2025-12-31T23:00:00Z' }), true, now).text).toBe('Due now')
  })
})
