import { describe, expect, it } from 'vitest'
import { flowResult, requestResult } from './rowmark'

describe('flowResult', () => {
  it('says nothing before a run', () => expect(flowResult(undefined)).toBe(''))
  it('says nothing while a run is going', () => expect(flowResult({ running: true, statuses: { a: 'passed' } })).toBe(''))
  it('is passed when every step passed or was skipped', () => {
    expect(flowResult({ running: false, statuses: { a: 'passed', b: 'skipped' } })).toBe('passed')
  })
  it('is failed when a step failed', () => {
    expect(flowResult({ running: false, statuses: { a: 'passed', b: 'failed' } })).toBe('failed')
  })
  it('is failed when the run ended with an error', () => {
    expect(flowResult({ running: false, error: 'no such step', statuses: {} })).toBe('failed')
  })
  it('says nothing for a run with no steps', () => expect(flowResult({ running: false, statuses: {} })).toBe(''))
})

describe('requestResult', () => {
  it('follows the checks of the response', () => {
    expect(requestResult(undefined)).toBe('')
    expect(requestResult({ passed: true })).toBe('passed')
    expect(requestResult({ passed: false })).toBe('failed')
  })
})
