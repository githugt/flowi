import { describe, expect, it } from 'vitest'
import { DEFAULT_EXISTING, EXISTING_CHOICES, groupKinds } from './importkinds'
import wizard from '../ImportScreen.svelte?raw'
import offer from '../PasteOffer.svelte?raw'

const ids = ['openapi', 'asyncapi', 'proto', 'arazzo', 'postman', 'bruno', 'insomnia', 'thunderclient', 'hoppscotch', 'openrpc', 'har', 'http', 'curl', 'mermaid']
const kinds = ids.map((id) => ({ id, label: id }))

describe('import kinds', () => {
  it('groups the 14 kinds and loses none', () => {
    // Before: one flat select of 14.
    const groups = groupKinds(kinds)

    expect(groups.map((g) => g.title)).toEqual(['Descriptions', 'Other tools', 'Captures', 'Sketches'])
    expect(groups.flatMap((g) => g.kinds.map((k) => k.id)).sort()).toEqual([...ids].sort())
  })

  it('puts a kind it does not know in More, and skips an empty group', () => {
    const groups = groupKinds([{ id: 'curl', label: 'curl' }, { id: 'new-thing', label: 'New' }])

    expect(groups.map((g) => g.title)).toEqual(['Captures', 'More'])
  })

  it('has one default for existing files, used by the wizard and the paste offer', () => {
    // Before: the wizard stopped at an existing file and the paste offer kept it.
    expect(DEFAULT_EXISTING).toBe('skip')
    expect(EXISTING_CHOICES[0].id).toBe(DEFAULT_EXISTING)
    expect(wizard).toContain('$state(DEFAULT_EXISTING)')
    expect(offer).toContain('$state(DEFAULT_EXISTING)')
    expect(wizard).toContain('groupKinds(view.kinds)')
  })
})
