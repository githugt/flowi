import { describe, expect, it } from 'vitest'
import { paramDoc } from './paramdoc'

describe('paramDoc', () => {
  it('gives the registry text of a field', () => {
    expect(paramDoc({ kind: 'text', doc: ' The URL to wait for. ' })).toBe('The URL to wait for.')
  })
  it('gives nothing for an expression field or a missing text', () => {
    expect(paramDoc({ kind: 'expr', doc: 'True or false.' })).toBe('')
    expect(paramDoc({ kind: 'text' })).toBe('')
  })
})
