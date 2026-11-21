import type { NamedAuth } from './types'

/** What the form of a named authentication holds until Save. */
export interface AuthDraft {
  description: string
  scheme: string
  values: Record<string, string>
}

/** The draft a form starts from: the stored authentication as it is. */
export function draftOf(a: NamedAuth): AuthDraft {
  return { description: a.description, scheme: a.scheme, values: Object.fromEntries(a.fields.map((f) => [f.name, f.value])) }
}

/**
 * Whether the form holds changes that are not saved: a description, a scheme or a value that differs from the stored
 * authentication. A value left empty is the same as one that is not stored.
 * Before: opening another authentication replaced the form, and what was typed in the first was lost without a word.
 */
export function authDirty(a: NamedAuth, d: AuthDraft): boolean {
  if (d.description !== a.description || d.scheme !== a.scheme) return true
  const stored = Object.fromEntries(a.fields.map((f) => [f.name, f.value]))
  return Object.keys({ ...stored, ...d.values }).some((n) => (stored[n] ?? '') !== (d.values[n] ?? ''))
}
