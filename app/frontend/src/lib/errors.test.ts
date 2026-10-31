import { describe, expect, it } from 'vitest'
import { errorText, reportUnhandled } from './errors'

class NamedError extends Error {
  constructor(name: string, message: string) {
    super(message)
    this.name = name
  }
}

describe('errorText', () => {
  it.each([
    ['an Error', new Error('timeout must be a duration like 5s or 1m'), 'timeout must be a duration like 5s or 1m'],
    ['a RuntimeError from Go', new NamedError('RuntimeError', 'unknown field "x"'), 'unknown field "x"'],
    ['a string with the Error prefix', 'Error: no such request', 'no such request'],
    ['a plain string', 'no such request', 'no such request'],
  ])('should give the message of %s', (_, e, want) => {
    const got = errorText(e)

    expect(got).toBe(want)
  })
})

describe('reportUnhandled', () => {
  it.each([
    ['a failed call', new NamedError('RuntimeError', 'the folder file is not valid YAML'), [['the folder file is not valid YAML', 'error']]],
    ['an error with no message', new Error(''), [['Something went wrong.', 'error']]],
    ['a cancelled call', new NamedError('CancelError', 'cancelled'), []],
    ['a rejection inside a cancelled call', new NamedError('CancelledRejectionError', 'cancelled'), []],
  ])('Given %s Then it shows what it should', (_, reason, want) => {
    const shown: string[][] = []

    reportUnhandled(reason, (text, level) => shown.push([text, level]))

    expect(shown).toEqual(want)
  })
})
