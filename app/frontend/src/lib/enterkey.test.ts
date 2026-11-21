import { describe, expect, it } from 'vitest'
import { isEnter } from './enterkey'

describe('isEnter', () => {
  // Before: handlers tested e.key === 'Enter' only, so confirming an IME conversion also ran the action.
  it.each([
    ['a plain Enter', { key: 'Enter' }, true],
    ['Enter that confirms a composition', { key: 'Enter', isComposing: true }, false],
    ['Enter reported as keyCode 229 (Safari)', { key: 'Enter', keyCode: 229 }, false],
    ['Enter with keyCode 13', { key: 'Enter', keyCode: 13, isComposing: false }, true],
    ['another key', { key: 'a' }, false],
  ])('Given %s Then %s', (_, e, want) => {
    expect(isEnter(e)).toBe(want)
  })
})
