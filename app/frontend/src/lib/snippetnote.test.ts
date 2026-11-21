import { describe, expect, it } from 'vitest'
import { snippetCopied, copyCodeLabel } from './snippetnote'

describe('snippetCopied', () => {
  it('warns when real secrets are in the snippet', () => expect(snippetCopied(true, false)).toBe('the snippet, with your real secrets in it'))
  it('says secrets were left out when they were', () => expect(snippetCopied(false, true)).toBe('the snippet (secrets left out)'))
  it('says only what was copied when there is nothing to say about secrets', () => expect(snippetCopied(false, false)).toBe('the snippet'))
})

describe('copyCodeLabel', () => {
  it('names the format the button copies', () => {
    // Before: Copy-as-cURL was hidden in More and the Code tab had to be opened first.
    expect(copyCodeLabel('curl')).toBe('Copy as curl')
  })
  it('falls back to a plain label when no format is known', () => expect(copyCodeLabel('')).toBe('Copy as code'))
})
