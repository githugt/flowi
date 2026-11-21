// The import wizard's kinds in groups, and the one default for files that already exist.
// Before: 14 kinds in one flat list, and the wizard stopped at an existing file while the paste offer kept the old one.

export interface KindLike {
  id: string
  label: string
}

const GROUPS: { title: string; ids: string[] }[] = [
  { title: 'Descriptions', ids: ['openapi', 'asyncapi', 'proto', 'openrpc', 'arazzo'] },
  { title: 'Other tools', ids: ['postman', 'bruno', 'insomnia', 'thunderclient', 'hoppscotch'] },
  { title: 'Captures', ids: ['har', 'http', 'curl'] },
  { title: 'Sketches', ids: ['mermaid'] },
]

/** The kinds in groups, in the order of GROUPS; a kind in no group goes in "More" so none is lost. */
export function groupKinds<T extends KindLike>(kinds: T[]): { title: string; kinds: T[] }[] {
  const placed = new Set<string>()
  const out = GROUPS.map((g) => ({
    title: g.title,
    kinds: g.ids.flatMap((id) => kinds.filter((k) => k.id === id)).map((k) => (placed.add(k.id), k)),
  })).filter((g) => g.kinds.length)
  const rest = kinds.filter((k) => !placed.has(k.id))
  return rest.length ? [...out, { title: 'More', kinds: rest }] : out
}

/** What to do with a file the import would write when it already exists: keep mine, add only the new ones. */
export const DEFAULT_EXISTING = 'skip'

export const EXISTING_CHOICES: { id: string; label: string }[] = [
  { id: 'skip', label: 'Keep mine, add only new files' },
  { id: 'overwrite', label: 'Overwrite them' },
  { id: 'refuse', label: 'Stop if any exist' },
]
