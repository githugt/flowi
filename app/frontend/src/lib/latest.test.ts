import { describe, expect, it } from 'vitest'
import { latest } from './latest'

describe('latest', () => {
  it('is true for the newest load only', () => {
    const start = latest()

    const first = start()
    const second = start()

    expect(first()).toBe(false)
    expect(second()).toBe(true)
  })
  it('keeps a lone load current however many times it is asked', () => {
    const current = latest()()

    expect(current()).toBe(true)
    expect(current()).toBe(true)
  })
  it('keeps separate sets of loads apart', () => {
    const coverage = latest()
    const change = latest()

    const c = coverage()
    change()
    change()

    expect(c()).toBe(true)
  })
})
