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
