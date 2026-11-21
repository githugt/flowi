import { describe, expect, it } from 'vitest'
import { subflowNotes } from './subflownotes'

const inputs = [{ name: 'user', type: 'string', required: true }, { name: 'page', type: 'int', default: '1' }]

describe('subflowNotes', () => {
  it('lists the inputs and the outputs as a later step reads them', () => {
    const n = subflowNotes('signin', ['user'], inputs, [{ name: 'token' }])

    expect(n.inputs).toEqual(['user (string, required)', 'page (int, default 1)'])
    expect(n.outputs).toEqual(['nodes.signin.token'])
  })
  it('names an input the flow does not have, and a required one that is not set', () => {
    // Before: both were found only when the flow ran.
    const n = subflowNotes('s', ['usr', 'page'], inputs)

    expect(n.unknown).toEqual(['usr'])
    expect(n.missing).toEqual(['user'])
  })
  it('flags nothing when the called flow is not known', () => {
    expect(subflowNotes('s', ['anything'])).toEqual({ unknown: [], missing: [], inputs: [], outputs: [] })
  })
})
