// Development status and tags of requests, flows and folders: how a status is drawn, and how a search reads
// "status:review tag:smoke" next to plain words. The status, its colour and its icon come from Go (lib/model/status.go);
// this holds only what the window decides.
import { fold } from './fold'
import type { StatusMark } from './types'

/** What a search says: plain words, `tag:x` / `#x` terms (all must hold) and `status:y` terms (any may hold). */
export interface LabelQuery {
  words: string[]
  tags: string[]
  statuses: string[]
}

/** What an item carries that a search can match. */
export interface Labelled {
  status?: StatusMark
  tags?: string[]
}

/**
 * Reads a search box. `tag:smoke` and `#smoke` ask for a tag, `status:review` for a status; anything else is a word.
 * A term with nothing after it ("tag:") is dropped, so it does not empty the list while it is being typed.
 */
export function parseQuery(query: string): LabelQuery {
  const out: LabelQuery = { words: [], tags: [], statuses: [] }
  for (const raw of query.split(/\s+/).filter(Boolean)) {
    const token = fold(raw)
    const tag = token.startsWith('#') ? token.slice(1) : token.startsWith('tag:') ? token.slice(4) : null
    if (tag !== null) {
      if (tag) out.tags.push(tag)
    } else if (token.startsWith('status:')) {
      const status = token.slice(7)
      if (status) out.statuses.push(status)
    } else out.words.push(token)
  }
  return out
}

/** True when the query asks for a tag or a status, not only words. */
export const hasLabelTerms = (q: LabelQuery): boolean => q.tags.length > 0 || q.statuses.length > 0

/**
 * Whether an item has the tags and a status the query asks for. A term is a prefix, so `tag:sm` and `status:rev` find
 * "smoke" and "review" while they are typed. Every tag term must hold; the status terms are alternatives, because an
 * item has one status. Words are not looked at here.
 */
export function labelsMatch(item: Labelled, q: LabelQuery): boolean {
  const tags = (item.tags ?? []).map(fold)
  if (!q.tags.every((t) => tags.some((have) => have.startsWith(t)))) return false
  if (!q.statuses.length) return true
  const status = item.status ? fold(item.status.name) : ''
  return !!status && q.statuses.some((s) => status.startsWith(s))
}

/** What a word search reads of an item beside its title: the status name and the tags, as they are written. */
export function labelText(item: Labelled): string {
  return [item.status?.name, ...(item.tags ?? [])].filter(Boolean).join(' ')
}

/** "in-development" read as words, for a tooltip and a screen reader. */
export const statusWords = (name: string): string => name.replace(/[-_]+/g, ' ')

/** The CSS colour of a status colour name: its theme token, so it is the one checked for contrast on every theme. */
export const statusColor = (color: string): string => `var(--st-${color || 'gray'})`

/** What a status mark says when hovered, and to a screen reader: its name, where it comes from, and a typo. */
export function statusTitle(m: StatusMark): string {
  const words = statusWords(m.name)
  if (m.unknown) return `Status “${words}” is not one of this collection’s statuses`
  if (m.inherited) return `Status: ${words} (from the folder ${m.from?.split('/').slice(1).join('/') || m.from})`
  return `Status: ${words}`
}

/** A tag as the files keep it: one word (white space and commas become '-'); "" for nothing. Same rule as the library's. */
export function cleanTag(tag: string): string {
  return tag
    .replace(/,/g, ' ')
    .trim()
    .split(/\s+/)
    .filter(Boolean)
    .join('-')
    .replace(/^-+|-+$/g, '')
}

/** The tags the person gave, cleaned, without empties or repeats, in the order given. */
export function cleanTags(tags: string[]): string[] {
  const out: string[] = []
  for (const t of tags.map(cleanTag)) if (t && !out.includes(t)) out.push(t)
  return out
}

/** A status name as the files keep it: lower case, spaces as '-'. The library refuses what is left that is not allowed. */
export const cleanStatusName = (name: string): string => name.trim().toLowerCase().split(/\s+/).join('-')

/** The tags in use that the person has not added yet and that fit what they typed, most used first, at most `max`. */
export function suggestTags(suggest: { tag: string; count: number }[], have: string[], typed: string, max = 8): { tag: string; count: number }[] {
  const t = fold(cleanTag(typed))
  return suggest.filter((s) => !have.includes(s.tag) && (!t || fold(s.tag).includes(t))).slice(0, max)
}

/** The statuses and tags that rows of the tree carry, for the filter menu: statuses in the order met, tags most used first. */
export function labelChoices(roots: { status?: StatusMark; tags?: string[]; children?: unknown[] }[]): { statuses: StatusMark[]; tags: { tag: string; count: number }[] } {
  const statuses = new Map<string, StatusMark>()
  const tags = new Map<string, number>()
  const walk = (nodes: { status?: StatusMark; tags?: string[]; children?: unknown[] }[]) => {
    for (const n of nodes) {
      if (n.status && !statuses.has(n.status.name)) statuses.set(n.status.name, n.status)
      for (const t of n.tags ?? []) tags.set(t, (tags.get(t) ?? 0) + 1)
      if (n.children?.length) walk(n.children as typeof nodes)
    }
  }
  walk(roots)
  return {
    statuses: [...statuses.values()],
    tags: [...tags].map(([tag, count]) => ({ tag, count })).sort((a, b) => b.count - a.count || a.tag.localeCompare(b.tag)),
  }
}

/** The query with a term added, or taken away when it is there already (the filter menu's toggle). */
export function toggleTerm(query: string, term: string): string {
  const parts = query.split(/\s+/).filter(Boolean)
  const at = parts.findIndex((p) => fold(p) === fold(term))
  if (at >= 0) parts.splice(at, 1)
  else parts.push(term)
  return parts.join(' ')
}
