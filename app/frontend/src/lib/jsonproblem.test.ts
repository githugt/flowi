import { describe, expect, it } from 'vitest'
import { jsonProblem } from './jsonproblem'

describe('jsonProblem', () => {
  it('Given valid JSON Then there is no problem', () => {
    expect(jsonProblem('{"a": [1, 2], "b": null}')).toBeNull()
  })
  it('Given an empty body Then there is no problem', () => {
    expect(jsonProblem('  \n')).toBeNull()
  })
  it('Given a template as a value Then it counts as a value', () => {
    expect(jsonProblem('{"id": {{user.id}}, "name": "{{user.name}}"}')).toBeNull()
  })
  it('Given a template inside a string Then it is part of the string', () => {
    expect(jsonProblem('{"q": "a {{ not closed"}')).toBeNull()
  })
  it('Given a missing comma Then it says where the text stops reading', () => {
    const got = jsonProblem('{\n  "a": 1\n  "b": 2\n}')
    expect(got).not.toBeNull()
    expect(got!.line).toBe(3)
  })
  it('Given an unfinished body Then the problem is at its end', () => {
    const got = jsonProblem('{"a": [1, 2')
    expect(got).not.toBeNull()
    expect(got!.line).toBe(1)
  })
})
