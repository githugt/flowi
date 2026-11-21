import { describe, expect, it } from 'vitest'
import { windowTitle } from './wintitle'

describe('windowTitle', () => {
  // Before: the title never changed from "Flowi", so a screen reader and the window list could not tell the tabs apart.
  it('names the open tab before the product', () => {
    expect(windowTitle('Login', false)).toBe('Login — Flowi')
  })
  it('marks unsaved edits', () => {
    expect(windowTitle('Login', true)).toBe('● Login — Flowi')
  })
  it('is the product name when nothing is open', () => {
    expect(windowTitle('  ', false)).toBe('Flowi')
  })
})
