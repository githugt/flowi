import { describe, expect, it } from 'vitest'
import { inputsToSend, lastInputs, rememberInputs } from './runinputs'

describe('inputsToSend', () => {
  it('leaves out a blank box so the default applies', () => {
    expect(inputsToSend({ a: 'x', b: '' }, [])).toEqual({ a: 'x' })
  })
  it('sends an input marked empty as the empty text', () => {
    // Before: the empty box was dropped, so "empty email" could not be sent.
    expect(inputsToSend({ email: '', name: 'Ann' }, ['email'])).toEqual({ name: 'Ann', email: '' })
  })
  it('lets the empty mark win over typed text', () => {
    expect(inputsToSend({ email: 'a@b' }, ['email'])).toEqual({ email: '' })
  })
})

describe('lastInputs', () => {
  it('gives the values of the last run of that flow, for the inputs it still has', () => {
    rememberInputs('f:signup', { email: 'a@b', old: 'x' }, ['name', 'gone'])

    // Before: the dialog started blank each time.
    expect(lastInputs('f:signup', ['email', 'name'])).toEqual({ values: { email: 'a@b' }, empty: ['name'] })
  })
  it('gives nothing for a flow that has not run', () => {
    expect(lastInputs('f:never', ['a'])).toEqual({ values: {}, empty: [] })
  })
  it('keeps a copy, so editing the dialog does not change what was remembered', () => {
    const v = { a: '1' }
    rememberInputs('f:copy', v, [])
    v.a = '2'

    expect(lastInputs('f:copy', ['a']).values.a).toBe('1')
  })
})
