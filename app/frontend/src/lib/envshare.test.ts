import { describe, expect, it } from 'vitest'
import { replaceShareText, sharedNotice } from './envshare'

describe('replaceShareText', () => {
  it('Given a file already in the collection When asking Then it names both and offers Replace', () => {
    const d = replaceShareText('staging', 'Payments')

    expect(d.text).toContain('Payments already has an environment file named staging')
    expect(d.action).toBe('Replace')
  })
})

describe('sharedNotice', () => {
  it('Given a share When noticed Then it says secrets went as names only', () => {
    // Before: environments could be imported from a collection but never exported to one.
    expect(sharedNotice('staging', 'Payments', [])).toBe('Shared staging with Payments. Its secrets went as names only; each person sets their own values.')
  })
  it('Given notes When noticed Then they follow the sentence', () => {
    expect(sharedNotice('child', 'P', ['child builds on base.'])).toContain('child builds on base.')
  })
})
