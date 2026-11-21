import { describe, expect, it } from 'vitest'
import { elapsedText } from './elapsed'

describe('elapsedText', () => {
  it('shows tenths of a second under 10 s', () => expect(elapsedText(420)).toBe('0.4 s'))
  it('shows whole seconds under a minute', () => expect(elapsedText(12_900)).toBe('12 s'))
  it('shows minutes and padded seconds after that', () => expect(elapsedText(65_000)).toBe('1 m 05 s'))
  it('does not go below zero', () => expect(elapsedText(-5)).toBe('0.0 s'))
})
