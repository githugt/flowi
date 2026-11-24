// What the sidebar's filter popover offers, and how its choices read as the search terms the field already takes
// (`status:review`, `tag:smoke`, `type:grpc`, `in:hyperverge`, `has:problems`; see lib/devstatus). A choice adds or takes
// away its term, so the popover and the typed text are one thing.
// Before: the filter menu offered statuses and tags only.
import { fold } from './fold'
import { placeSlug, typeTerm } from './devstatus'
import type { StatusMark, TreeNode } from './types'

export interface TypeChoice {
  id: string
  /** What is typed after `type:`. */
  term: string
  name: string
  count: number
}

export interface FilterChoices {
  statuses: StatusMark[]
  tags: { tag: string; count: number }[]
  types: TypeChoice[]
  /** Collections, in the order the tree has them. */
  collections: { name: string; term: string; count: number }[]
}

const TYPE_ORDER = ['http', 'grpc', 'message', 'db', 'file', 'flow']

/** What the rows carry that a filter can pick: the statuses and tags, the types of request, and the collections. */
export function filterChoices(roots: TreeNode[]): FilterChoices {
  const statuses = new Map<string, StatusMark>()
  const tags = new Map<string, number>()
  const types = new Map<string, TypeChoice>()
  const collections = new Map<string, number>()
  const walk = (nodes: TreeNode[], collection: string) => {
    for (const n of nodes) {
      const here = n.kind === 'collection' && !n.scratch ? n.label : n.kind === 'collection' ? 'Drafts' : collection
      if (n.status && !statuses.has(n.status.name)) statuses.set(n.status.name, n.status)
      for (const t of n.tags ?? []) tags.set(t, (tags.get(t) ?? 0) + 1)
      if ((n.kind === 'request' || n.kind === 'flow') && n.type) {
        const have = types.get(n.type)
        if (have) have.count++
        else types.set(n.type, { id: n.type, term: typeTerm(n.type), name: n.typeName ?? n.type, count: 1 })
        collections.set(here, (collections.get(here) ?? 0) + 1)
      } else if (n.kind === 'collection' && !collections.has(here)) collections.set(here, 0)
      if (n.children?.length) walk(n.children, here)
    }
  }
  walk(roots, '')
  return {
    statuses: [...statuses.values()],
    tags: [...tags].map(([tag, count]) => ({ tag, count })).sort((a, b) => b.count - a.count || a.tag.localeCompare(b.tag)),
    types: [...types.values()].sort((a, b) => TYPE_ORDER.indexOf(a.id) - TYPE_ORDER.indexOf(b.id)),
    collections: [...collections].map(([name, count]) => ({ name, term: placeSlug(name), count })),
  }
}

/** The terms of a query that are filters (not plain words), as typed. */
export function filterTerms(query: string): string[] {
  return query.split(/\s+/).filter((p) => /^(#|tag:|status:|type:|in:|has:)./i.test(p))
}

/** Whether the query has this term (case does not matter). */
export const hasTerm = (query: string, term: string): boolean => query.split(/\s+/).some((p) => fold(p) === fold(term))

/** The query without any filter term: the plain words the person typed stay. */
export const withoutFilters = (query: string): string => query.split(/\s+/).filter((p) => p && !filterTerms(p).length).join(' ')

/** The query with the terms that start with `prefix` ("type:") taken out: "All" in a group of choices. */
export const withoutPrefix = (query: string, prefix: string): string => query.split(/\s+/).filter((p) => p && !fold(p).startsWith(fold(prefix))).join(' ')

/** A short line for the filter button when filters are on: "2 filters". */
export function filtersNote(query: string): string {
  const n = filterTerms(query).length
  return n === 0 ? '' : n === 1 ? '1 filter on' : `${n} filters on`
}
