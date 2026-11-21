import { describe, expect, it } from 'vitest'
import { checkNotes, checkStates, checkTally, triedStates } from './checks'

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

describe('checkStates when a note names a longer check', () => {
  // Before: a note about `status == 2000` also failed `status == 200`, because the text was a substring of it.
  const diags = ['FLW-RUN-001: assertion failed: status == 2000']
  const outcome = { status: 'failed', diags, checks: ['status == 200', 'status == 2000'] }

  it('should fail only the check the note names', () => {
    expect(checkStates(['status == 200', 'status == 2000'], outcome)).toEqual(['passed', 'failed'])
  })

  it('should give a check only the notes about it', () => {
    expect(checkNotes('status == 200', outcome)).toEqual([])
    expect(checkNotes('status == 2000', outcome)).toEqual(diags)
  })
})

describe('triedStates', () => {
  const sent = { status: 'passed', diags: [], checks: ['status == 200'] }

  // Before: a check added or edited after a send stayed a grey circle until the request was sent again.
  it('should judge a check the send did not judge by trying it on the shown response', () => {
    const checks = ['status == 200', 'status < 300', 'body.id > 0', 'vars.t != empty']
    const states = checkStates(checks, sent)

    const got = triedStates(checks, states, [
      { line: 'status < 300', verdict: 'passed', note: '' },
      { line: 'body.id > 0', verdict: 'failed', note: 'body.id = 0' },
      { line: 'vars.t != empty', verdict: 'unknown', note: 'vars.t is not defined' },
    ])

    expect(got).toEqual([
      { state: 'passed', tried: false, note: '' },
      { state: 'passed', tried: true, note: '' },
      { state: 'failed', tried: true, note: 'body.id = 0' },
      { state: 'idle', tried: false, note: 'vars.t is not defined' },
    ])
  })

  it('should not overrule what the send said about a check it judged', () => {
    const states = checkStates(['status == 200'], { status: 'failed', diags: ['FLW-RUN-001: assertion failed: status == 200'], checks: ['status == 200'] })

    const got = triedStates(['status == 200'], states, [{ line: 'status == 200', verdict: 'passed', note: '' }])

    expect(got[0]).toEqual({ state: 'failed', tried: false, note: '' })
  })
})

describe('checkStates with what the send reported', () => {
  const results = [
    { line: 'status == 200', verdict: 'passed', note: '' },
    { line: 'body.id > 0', verdict: 'failed', note: 'body.id = 0' },
    { line: 'bad ==', verdict: 'error', note: 'syntax' },
  ]

  // Before: a pass was inferred from the absence of a note that named the check.
  it('should take each state from the send, and leave a check it did not reach untried', () => {
    const checks = ['status == 200', 'body.id > 0', 'bad ==', 'status < 300']

    const got = checkStates(checks, { status: 'failed', diags: [], checks, checkResults: results })

    expect(got).toEqual(['passed', 'failed', 'error', 'idle'])
  })

  it('should fall back to the notes when the send reported nothing', () => {
    const got = checkStates(['status == 200'], { status: 'passed', diags: [], checks: ['status == 200'], checkResults: [] })

    expect(got).toEqual(['passed'])
  })

  it('should tally the passed checks, and say nothing without results', () => {
    expect(checkTally({ checkResults: results })).toEqual({ passed: 1, total: 3, text: '1/3 checks', ok: false })
    expect(checkTally({ checkResults: [results[0]] })?.text).toBe('1/1 check')
    expect(checkTally({ checkResults: [] })).toBeUndefined()
    expect(checkTally(undefined)).toBeUndefined()
  })
})
