import { describe, expect, it } from 'vitest'
import { tookTone } from './took'

describe('tookTone', () => {
  it.each([
    ['820 µs', ''],
    ['142 ms', ''],
    ['500 ms', ''],
    ['501 ms', 'slow'],
    ['1.24 s', 'slow'],
    ['2.00 s', 'slow'],
    ['2.01 s', 'verySlow'],
    ['12.50 s', 'verySlow'],
    ['', ''],
    ['soon', ''],
  ])('%s reads as %j', (text, tone) => {
    expect(tookTone(text)).toBe(tone)
  })
})

describe('tookTone without a space', () => {
  it('reads Go durations as the replayer writes them ("142ms", "2.4s")', () => {
    // Before: only "142 ms" was read, so the replayer's times were never tinted.
    expect(tookTone('142ms')).toBe('')
    expect(tookTone('900ms')).toBe('slow')
    expect(tookTone('2.4s')).toBe('verySlow')
  })
})
