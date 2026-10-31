import { describe, expect, it } from 'vitest'
import { codeParts, describeProblem, describeProblems } from './problems'

describe('describeProblem', () => {
  it.each([
    ['FLW-TPL-001: undefined `env.base_url` in URL of auth/login', '`base_url` is not set in the environment (used in the URL of auth/login).'],
    [
      'FLW-TPL-001: undefined `secret.password` in body of auth/login (no secret variable password in this environment)',
      'The secret `password` is not set in the environment (used in the body of auth/login): no secret variable password in this environment.',
    ],
    ['FLW-TPL-001: undefined `email` in body of auth/login', '`email` has no value (used in the body of auth/login).'],
    ['FLW-TPL-001: undefined `$nope` in URL of x', '`$nope` is not a known function (used in the URL of x).'],
  ])('puts %s in plain words, code last', (raw, text) => {
    expect(describeProblem(raw)).toEqual({ text, code: 'FLW-TPL-001' })
  })

  it('keeps other coded messages as they are, and uncoded text whole', () => {
    expect(describeProblem('FLW-REQ-004: unknown environment "x"')).toEqual({ text: 'unknown environment "x"', code: 'FLW-REQ-004' })
    expect(describeProblem('  connection refused ')).toEqual({ text: 'connection refused', code: '' })
  })
})

describe('codeParts', () => {
  it('marks the parts between backticks as code', () => {
    expect(codeParts('a `b` c')).toEqual([{ text: 'a ', code: false }, { text: 'b', code: true }, { text: ' c', code: false }])
  })
})

describe('describeProblems', () => {
  it('splits a run error that joins several problems', () => {
    const raw = 'FLW-TPL-001: undefined `env.base_url` in URL of auth/login FLW-TPL-001: undefined `secret.password` in body of auth/login'

    expect(describeProblems(raw)).toEqual([
      { text: '`base_url` is not set in the environment (used in the URL of auth/login).', code: 'FLW-TPL-001' },
      { text: 'The secret `password` is not set in the environment (used in the body of auth/login).', code: 'FLW-TPL-001' },
    ])
  })

  it('keeps text before the first code as a problem of its own', () => {
    expect(describeProblems('run cancelled')).toEqual([{ text: 'run cancelled', code: '' }])
  })
})
