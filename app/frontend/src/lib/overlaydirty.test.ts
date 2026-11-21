import { describe, expect, it } from 'vitest'
import { overlayDirty } from './overlaydirty'

describe('overlayDirty', () => {
  it('is false when nothing is open', () => expect(overlayDirty(false, false, '', 'x', '')).toBe(false))
  it('is false for an overlay as it was read', () => expect(overlayDirty(true, false, 'vip', 'a: 1', 'a: 1')).toBe(false))
  it('is true once the text changes', () => expect(overlayDirty(true, false, 'vip', 'a: 2', 'a: 1')).toBe(true))
  it('is true for a new overlay that has a name', () => expect(overlayDirty(true, true, 'slow', 'a: 1', 'a: 1')).toBe(true))
  it('is false for a new overlay left as the template', () => expect(overlayDirty(true, true, '  ', 'a: 1', 'a: 1')).toBe(false))
})
