import { describe, expect, it } from 'vitest'
import { stepAtLine } from './codeStep'

const flow = `name: Signup
inputs:
  email: {}
nodes:
  - id: login
    type: request
    request:
      id: 01J
  - type: note
    id: "say"
    text: hi

  - id: last
    type: delay
`

describe('stepAtLine', () => {
  it('finds the step a line is inside, however deep', () => {
    expect(stepAtLine(flow, 5)).toBe('login')
    expect(stepAtLine(flow, 8)).toBe('login')
    expect(stepAtLine(flow, 10)).toBe('say')
    expect(stepAtLine(flow, 14)).toBe('last')
  })
  it('finds an id that is not on the first line of the step', () => {
    expect(stepAtLine(flow, 9)).toBe('say')
  })
  it('keeps the previous step for a blank line between steps', () => {
    expect(stepAtLine(flow, 12)).toBe('say')
  })
  it('says nothing outside the list of steps', () => {
    expect(stepAtLine(flow, 1)).toBe('')
    expect(stepAtLine(flow, 3)).toBe('')
    expect(stepAtLine(flow, 4)).toBe('')
  })
  it('says nothing for a flow without steps', () => {
    expect(stepAtLine('name: x\nnodes: []\n', 2)).toBe('')
    expect(stepAtLine('', 1)).toBe('')
  })
  it('ignores a step list that ends before a later top-level key', () => {
    expect(stepAtLine('nodes:\n  - id: a\n    type: note\noutputs:\n  x: 1\n', 5)).toBe('')
  })
})
