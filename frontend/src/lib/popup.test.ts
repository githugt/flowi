import { describe, expect, it } from 'vitest'
import { filterSteps, groupSteps, stepAt, nextIndex, withRouteByStatus } from './popup'
import type { StepChoice } from './types'

const steps: StepChoice[] = [
  { group: 'Call', name: 'Request', description: 'Send a saved request, or call a URL', type: 'request' },
  { group: 'Call', name: 'Flow', description: 'Run another flow with its inputs', type: 'subflow' },
  { group: 'Branch', name: 'Route', description: 'Take a different path by status or by value', type: 'condition' },
  { group: 'Branch', name: 'Loop', description: 'Repeat steps over a list', type: 'batch' },
]

describe('filterSteps', () => {
  it('lists every step for a blank search', () => {
    expect(filterSteps(steps, '  ')).toHaveLength(4)
  })
  it('matches the name, ignoring case', () => {
    expect(filterSteps(steps, 'ROUT').map((s) => s.name)).toEqual(['Route'])
  })
  it('matches a word of the description', () => {
    expect(filterSteps(steps, 'repeat').map((s) => s.name)).toEqual(['Loop'])
  })
  it('needs every word to match', () => {
    expect(filterSteps(steps, 'send url').map((s) => s.name)).toEqual(['Request'])
    expect(filterSteps(steps, 'send loop')).toEqual([])
  })
})

describe('groupSteps', () => {
  it('keeps the order and starts a group when the name changes', () => {
    expect(groupSteps(steps).map((g) => [g.name, g.items.length])).toEqual([['Call', 2], ['Branch', 2]])
  })
})

describe('nextIndex', () => {
  it('moves down and wraps to the top', () => {
    expect(nextIndex(0, 1, 4)).toBe(1)
    expect(nextIndex(3, 1, 4)).toBe(0)
  })
  it('moves up and wraps to the bottom', () => {
    expect(nextIndex(0, -1, 4)).toBe(3)
  })
  it('stays at zero when nothing is listed', () => {
    expect(nextIndex(0, 1, 0)).toBe(0)
  })
})

describe('stepAt', () => {
  it('finds the step at a position across the groups', () => {
    expect(stepAt(groupSteps(steps), 2)?.name).toBe('Route')
    expect(stepAt(groupSteps(steps), 9)).toBeUndefined()
  })
})

describe('withRouteByStatus', () => {
  it('adds the shortcut right after Route, in its group, naming the step', () => {
    const got = withRouteByStatus(steps, 'login')
    expect(got.map((s) => s.name)).toEqual(['Request', 'Flow', 'Route', 'Route by status', 'Loop'])
    expect(got[3]).toMatchObject({ group: 'Branch', type: 'condition', mode: 'status' })
    expect(got[3].description).toContain('login')
  })
  it('leaves the list alone when there is no step with a status before', () => {
    expect(withRouteByStatus(steps, '')).toBe(steps)
  })
  it('is found by searching for status', () => {
    expect(filterSteps(withRouteByStatus(steps, 'login'), 'status').map((s) => s.name)).toEqual(['Route', 'Route by status'])
  })
})
