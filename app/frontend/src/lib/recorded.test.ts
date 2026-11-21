import { describe, expect, it } from 'vitest'
import { recordedHeadline, recordedList } from './recorded'

describe('recordedHeadline', () => {
  it('Given saved requests When shown Then it counts them and names the place', () => {
    // Before: record mode ended with no result.
    expect(recordedHeadline({ count: 7, requests: [], target: 'Payments/recorded' })).toBe('Saved 7 new requests in Payments/recorded')
    expect(recordedHeadline({ count: 1, requests: [], target: 'Scratch/recorded' })).toBe('Saved 1 new request in Scratch/recorded')
  })
  it('Given nothing saved When shown Then there is no headline', () => {
    expect(recordedHeadline({ count: 0, requests: [], target: 'Scratch/recorded' })).toBe('')
  })
})

describe('recordedList', () => {
  it('Given many requests When listed Then the first few show and the rest are counted', () => {
    const requests = Array.from({ length: 11 }, (_, i) => `recorded/r${i}`)

    const got = recordedList({ count: 11, requests, target: 'x' })

    expect(got.shown).toHaveLength(8)
    expect(got.more).toBe(3)
  })
})
