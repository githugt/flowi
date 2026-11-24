import { describe, expect, it } from 'vitest'
import { stepNotes } from './stepnotes'

const flow = ['nodes:', '  - id: login', '    type: request', '  - id: "wait"', '    type: delay', '  - id: verify', '    type: request'].join('\n')

describe('stepNotes', () => {
  it('puts a note above the line of each step that has a comment, indented like the step', () => {
    expect(stepNotes(flow, { login: 'Login — POST /login', verify: 'Verify — POST /verify' })).toEqual([
      { line: 2, indent: 2, text: 'Login — POST /login' },
      { line: 6, indent: 2, text: 'Verify — POST /verify' },
    ])
  })
  it('finds a step whose id is quoted, and skips one with no comment', () => {
    expect(stepNotes(flow, { wait: 'Waits a second' })).toEqual([{ line: 4, indent: 2, text: 'Waits a second' }])
    expect(stepNotes(flow, {})).toEqual([])
  })
  it('has none when the view sent no comments', () => {
    expect(stepNotes(flow, undefined)).toEqual([])
  })
  it('finds steps inside a container too', () => {
    const nested = ['nodes:', '  - id: each', '    type: batch', '    body:', '      - id: call', '        type: request'].join('\n')

    expect(stepNotes(nested, { call: 'GET /x' })).toEqual([{ line: 5, indent: 6, text: 'GET /x' }])
  })
})
