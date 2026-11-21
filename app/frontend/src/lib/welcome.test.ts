import { describe, expect, it } from 'vitest'
import { MODEL_LINE, welcomeSteps } from './welcome'

const none = { collections: [], selectedEnv: '', environments: 0 }

describe('welcomeSteps', () => {
  // Before: three steps; none said what a collection is or that a check exists.
  it('explains what a collection is on a first run', () => {
    expect(welcomeSteps(none)[0].detail).toContain('A collection is a folder of YAML files')
  })
  it('ends with a step that adds a check', () => {
    const steps = welcomeSteps(none)
    expect(steps).toHaveLength(4)
    expect(steps[3].title).toBe('Add a check')
  })
  it('marks the first two steps from the state', () => {
    const steps = welcomeSteps({ collections: ['Shop'], selectedEnv: 'staging', environments: 1, firstRequest: 'Login', firstFlow: 'Signup' })
    expect(steps.map((s) => s.done)).toEqual([true, true, false, false])
    expect(steps[0].detail).toBe('Shop')
    expect(steps[2].detail).toBe('For example Login or the Signup flow.')
  })
  it('states the model: request, flow, check', () => {
    expect(MODEL_LINE.indexOf('Request')).toBeLessThan(MODEL_LINE.indexOf('Flow'))
    expect(MODEL_LINE.indexOf('Flow')).toBeLessThan(MODEL_LINE.indexOf('Check'))
  })
})
