import { describe, expect, it } from 'vitest'
import { checkStates } from './checks'

describe('checkStates', () => {
  it('should leave every check untried before a send', () => {
    const got = checkStates(['status == 200'], undefined)

    expect(got).toEqual(['idle'])
  })

  it('should pass the checks no note names after a send that ran them', () => {
    const got = checkStates(['status == 200', 'body.id > 0'], { status: 'passed', diags: [], checks: ['status == 200', 'body.id > 0'] })

    expect(got).toEqual(['passed', 'passed'])
  })

  it('should fail the check an assertion note names, and pass the other', () => {
    const diags = ['FLW-RUN-001: assertion failed: status == 200']

    const got = checkStates(['status == 200', 'body.id > 0'], { status: 'failed', diags, checks: ['status == 200', 'body.id > 0'] })

    expect(got).toEqual(['failed', 'passed'])
  })

  it('should mark a check that could not be evaluated as an error, not a pass', () => {
    const diags = ['body.funzies[1].id has no value (used in the assert body.funzies[1].id==23 of untitled). FLW-TPL-001']

    const got = checkStates(['status==200', 'body.funzies[1].id==23'], { status: 'failed', diags, checks: ['status==200', 'body.funzies[1].id==23'] })

    expect(got).toEqual(['passed', 'error'])
  })

  it('should not claim a pass when the send never reached the checks', () => {
    const got = checkStates(['status == 200'], { status: 'error', diags: [], checks: ['status == 200'] })

    expect(got).toEqual(['idle'])
  })

  it('should ignore blank checks', () => {
    const got = checkStates(['', 'status == 200'], { status: 'passed', diags: [], checks: ['', 'status == 200'] })

    expect(got).toEqual(['idle', 'passed'])
  })
})

describe('checkStates after the checks were changed', () => {
  const sent = { status: 'passed', diags: [] as string[], checks: ['status == 200'] }

  it('should show a check added since the send as untried, not passed', () => {
    const got = checkStates(['status == 200', 'body.id > 0'], sent)

    expect(got).toEqual(['passed', 'idle'])
  })

  it('should show an edited check as untried until it is sent again', () => {
    const got = checkStates(['status == 201'], sent)

    expect(got).toEqual(['idle'])
  })

  it('should ignore spaces around a check when matching what was sent', () => {
    const got = checkStates(['  status == 200 '], sent)

    expect(got).toEqual(['passed'])
  })

  it('should leave a check untried when the response does not say which checks it judged', () => {
    const got = checkStates(['status == 200'], { status: 'passed', diags: [], checks: [] })

    expect(got).toEqual(['idle'])
  })
})
