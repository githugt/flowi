import { describe, expect, it } from 'vitest'
import { showWrapBar, wrapKinds, wrapLabel } from './wrapbar'

describe('showWrapBar', () => {
  it('should show for two or more selected steps while editing, and not otherwise', () => {
    // Before: selecting several steps offered only Delete.
    expect(showWrapBar(true, 2)).toBe(true)
    expect(showWrapBar(true, 1)).toBe(false)
    expect(showWrapBar(false, 3)).toBe(false)
  })
})

describe('wrapLabel', () => {
  it('should count the steps in the right number', () => {
    expect(wrapLabel(3)).toBe('Wrap 3 steps in')
  })
})

describe('wrapKinds', () => {
  it('should offer the loop, the group and the route', () => {
    expect(wrapKinds.map((k) => k.kind)).toEqual(['loop', 'group', 'route'])
  })
})
