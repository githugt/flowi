import { describe, expect, it } from 'vitest'
import { footerLabel, overlayChipText } from './footer'

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

describe('overlayChipText', () => {
  it('counts the changes of every overlay in the stack and says when one is an identity', () => {
    const stack = [{ changes: ['a', 'b'], identity: true }, { changes: ['c'], identity: false }]

    expect(overlayChipText(stack)).toBe('3 overrides · identity')
  })

  it('says one override in the singular and leaves identity out when none is', () => {
    expect(overlayChipText([{ changes: ['a'], identity: false }])).toBe('1 override')
  })

  it('is empty without overlays', () => {
    expect(overlayChipText([])).toBe('')
  })
})
