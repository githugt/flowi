import { describe, expect, it } from 'vitest'
import { footerLabel } from './footer'

describe('footerLabel', () => {
  it('shows a version and an overlay stack, both', () => {
    expect(footerLabel('2', 'vip+slow')).toBe('Ver. 2 + vip + slow')
  })
  it('shows just the version when no overlay is laid over', () => {
    expect(footerLabel('2', '')).toBe('Ver. 2')
  })
  it('shows nothing for a tab that has no version', () => {
    expect(footerLabel('', '')).toBe('')
  })
})
