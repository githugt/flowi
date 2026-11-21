import { describe, expect, it } from 'vitest'
import { toastLife } from './toastlife'

describe('toastLife', () => {
  it('keeps a short note for 4 seconds and a short error for 8', () => {
    expect(toastLife('Copied', 'info')).toBe(4000)
    expect(toastLife('Could not save', 'error')).toBe(8000)
  })
  it('adds time for every character past 60', () => {
    expect(toastLife('x'.repeat(160), 'info')).toBe(4000 + 100 * 50)
  })
  it('stops at 20 seconds', () => {
    expect(toastLife('x'.repeat(5000), 'error')).toBe(20000)
  })
})
