import { describe, expect, it } from 'vitest'
import { authDirty, draftOf } from './authdraft'
import type { NamedAuth } from './types'

const auth = (): NamedAuth => ({
  key: 'C1/A1', collection: 'C1', collectionName: 'Shop', name: 'partner', description: 'The key', scheme: 'scheme:apikey', schemeLabel: 'API key', uses: [],
  fields: [{ name: 'name', value: 'X-Api-Key', secret: false, known: true }, { name: 'value', value: '', secret: true, known: true }],
})

describe('authDirty', () => {
  it('should be clean for the draft the form starts from', () => {
    expect(authDirty(auth(), draftOf(auth()))).toBe(false)
  })

  it('should be dirty when the description, the scheme or a value changes', () => {
    // Before: opening another authentication lost these edits without a word.
    const a = auth()
    expect(authDirty(a, { ...draftOf(a), description: 'Other' })).toBe(true)
    expect(authDirty(a, { ...draftOf(a), scheme: 'scheme:digest' })).toBe(true)
    expect(authDirty(a, { ...draftOf(a), values: { ...draftOf(a).values, value: '{{secret.k}}' } })).toBe(true)
  })

  it('should treat an empty value and an absent one as the same', () => {
    const a = auth()
    expect(authDirty(a, { ...draftOf(a), values: { name: 'X-Api-Key', extra: '' } })).toBe(false)
  })
})
