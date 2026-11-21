import { describe, expect, it } from 'vitest'
import { canSet, chipAt, exprOfChip, type ChipBox } from './chiphint'
import type { VariableHint } from './types'

const chip = (left: number, right: number, text = '{{env.a}}', unknown = false): ChipBox => ({ left, right, top: 10, bottom: 30, text, unknown })

describe('chipAt', () => {
  it('should find the chip whose box holds the point and none between or outside them', () => {
    // Before: the chips were drawn behind the input and nothing could tell which one the mouse was over.
    const chips = [chip(10, 50), chip(80, 120, '{{env.b}}')]

    expect(chipAt(chips, 20, 15)?.text).toBe('{{env.a}}')
    expect(chipAt(chips, 100, 29)?.text).toBe('{{env.b}}')
    expect(chipAt(chips, 60, 15)).toBeUndefined()
    expect(chipAt(chips, 20, 40)).toBeUndefined()
    expect(chipAt([], 20, 15)).toBeUndefined()
  })
})

describe('exprOfChip', () => {
  it('should take the braces and the spaces off', () => {
    expect(exprOfChip('{{env.base_url}}')).toBe('env.base_url')
    expect(exprOfChip('{{ $uuid-v4 }}')).toBe('$uuid-v4')
    expect(exprOfChip('{{env.open')).toBe('env.open')
  })
})

describe('canSet', () => {
  const hint = (over: Partial<VariableHint>): VariableHint => ({ text: '', known: false, environment: 'staging', name: 'x', secret: false, ...over })

  it('should allow a click only on an unset variable that has an environment and a name', () => {
    expect(canSet(hint({}))).toBe(true)
    expect(canSet(hint({ known: true }))).toBe(false)
    expect(canSet(hint({ environment: '' }))).toBe(false)
    expect(canSet(hint({ name: '' }))).toBe(false)
    expect(canSet(null)).toBe(false)
  })
})
