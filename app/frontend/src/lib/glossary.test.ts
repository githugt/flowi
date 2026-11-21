import { describe, expect, it } from 'vitest'
import { glossary } from './glossary'

describe('glossary', () => {
  it('explains each term in one sentence that names it', () => {
    for (const [term, text] of Object.entries(glossary)) {
      expect(text.length, term).toBeGreaterThan(30)
      expect(text.toLowerCase(), term).toContain(term)
      expect(text.endsWith('.'), term).toBe(true)
    }
  })
})
