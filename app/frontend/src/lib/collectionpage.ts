// The page of one collection: which tab node it has, and the words it uses for what the library measured.

/** The node of a collection's page in the tab strip: screen:collection:<id>, one tab for each collection. */
export const PAGE_PREFIX = 'screen:collection:'

export const pageNode = (collection: string): string => PAGE_PREFIX + collection

/** The collection a page tab belongs to; '' for any other node. */
export const pageCollection = (node: string): string => (node.startsWith(PAGE_PREFIX) ? node.slice(PAGE_PREFIX.length) : '')

/** The collection id a sidebar row stands for ("c:<id>" for a collection row); '' for a row of another kind. */
export const collectionOfRow = (row: { id: string; kind: string } | undefined): string =>
  row?.kind === 'collection' ? row.id.replace(/^c:/, '') : ''

export interface PageState {
  operationsUsed: number
  operationsTotal: number
  failing: number
  neverRan: number
  broken: string[]
  specTotal: number
  specErrors: number
  impactBroken: number
  driftMoved: number
}

export interface Headline {
  text: string
  /** ok: nothing to look at; bad: a flow fails or a file is unreadable; warn: something is not covered or never ran. */
  tone: 'ok' | 'warn' | 'bad'
}

const plural = (n: number, one: string, many = one + 's') => `${n} ${n === 1 ? one : many}`

/** One line on how the collection stands, worst first: what to look at before anything else. */
export function headline(s: PageState): Headline {
  const bits: string[] = []
  if (s.failing > 0) bits.push(`${plural(s.failing, 'flow')} failing`)
  if (s.broken.length > 0) bits.push(`${plural(s.broken.length, 'file')} that cannot be read`)
  if (s.specErrors > 0) bits.push(`${plural(s.specErrors, 'rule error')} in the API description`)
  if (s.neverRan > 0) bits.push(`${plural(s.neverRan, 'flow')} never ran`)
  const uncalled = s.operationsTotal - s.operationsUsed
  if (uncalled > 0) bits.push(`${plural(uncalled, 'operation')} no flow calls`)
  if (s.impactBroken > 0) bits.push(`${plural(s.impactBroken, 'operation')} that something reads wrongly`)
  if (s.driftMoved > 0) bits.push(`${plural(s.driftMoved, 'operation')} whose responses moved`)
  const warned = s.specTotal - s.specErrors
  if (warned > 0) bits.push(`${plural(warned, 'note')} on the API description`)
  if (bits.length === 0) return { text: 'Nothing needs a look.', tone: 'ok' }
  return { text: bits.join(' · '), tone: s.failing > 0 || s.broken.length > 0 || s.specErrors > 0 || s.impactBroken > 0 ? 'bad' : 'warn' }
}

/** The tone of a saved run's status in a Status pill. */
export function runTone(status: string): '' | 'ok' | 'bad' | 'warn' {
  if (status === 'passed') return 'ok'
  if (status === '') return ''
  if (status === 'skipped' || status === 'cancelled') return 'warn'
  return 'bad'
}

/** "3 of 5" for statuses a run got of the ones the description documents. */
export const ofTotal = (n: number, total: number): string => `${n} of ${total}`

export interface SpecRule {
  name: string
  about: string
  default: string
  level: string
  chosen: boolean
}

/** What a rule's select offers: its default first (so choosing it forgets the choice), then the three levels. */
export function levelChoices(rule: SpecRule): { value: string; label: string }[] {
  return [
    { value: '', label: `Default (${rule.default})` },
    { value: 'off', label: 'Off' },
    { value: 'warning', label: 'Warning' },
    { value: 'error', label: 'Error' },
  ]
}

/** The value a rule's select shows: '' when the collection chose nothing. */
export const levelShown = (rule: SpecRule): string => (rule.chosen ? rule.level : '')

/** Who reads an operation's responses and whether they still fit its contract (what the library's Impact says). */
export interface Impact {
  consumers: number
  broken: number
  unknown: number
  contract: string
  notes: string[]
}

/** What the last drift check on the page found for an operation. */
export interface Drift {
  checked: boolean
  at: string
  via: string[]
  new: number
  missing: number
  changed: number
  noSchema: boolean
  notes: string[]
}

export type Tone = '' | 'ok' | 'warn' | 'bad'

/** One line on who reads an operation, with the tone of its worst part. */
export function impactText(i: Impact): { text: string; tone: Tone } {
  if (i.consumers === 0) return { text: 'nothing reads its responses', tone: '' }
  const readers = plural(i.consumers, 'reader')
  if (i.broken > 0) return { text: `${readers}, ${i.broken} no longer fit${i.broken === 1 ? 's' : ''} its contract`, tone: 'bad' }
  if (i.unknown > 0) return { text: `${readers}, ${i.unknown} cannot be judged: no example or saved response`, tone: 'warn' }
  return { text: `${readers}, all fit its ${i.contract === 'example' ? 'pinned example' : 'last saved response'}`, tone: 'ok' }
}

/** The differences a drift check found, in all. */
export const driftMoved = (d: Drift): number => d.new + d.missing + d.changed

/** One line on what a drift check found; "not checked" until a flow that calls the operation was checked. */
export function driftText(d: Drift): { text: string; tone: Tone } {
  if (!d.checked) return { text: 'not checked', tone: '' }
  const moved = driftMoved(d)
  if (moved === 0) return d.noSchema ? { text: 'no stored schema to compare with', tone: 'warn' } : { text: 'no change', tone: 'ok' }
  const parts = [d.new && `${d.new} new`, d.missing && `${d.missing} missing`, d.changed && `${d.changed} changed`].filter(Boolean)
  return { text: `${moved} moved: ${parts.join(', ')}`, tone: d.missing + d.changed > 0 ? 'bad' : 'warn' }
}

export interface Endpoint {
  node: string
  path: string
  name: string
  summary: string
  verb: string
  where: string
  folder: string
  tags: string[]
  status?: { name: string; color: string; icon: string; from?: string; inherited?: boolean; unknown?: boolean }
  operation: string
  versions: string[]
  used: string[]
  statuses: { status: number; seen: boolean }[]
  unread: number
  impact: Impact
  drift: Drift
}

export type GroupBy = 'path' | 'tags' | 'folder' | 'status'

export const GROUPINGS: { id: GroupBy; label: string }[] = [
  { id: 'path', label: 'Paths' },
  { id: 'tags', label: 'Tags' },
  { id: 'folder', label: 'Folders' },
  { id: 'status', label: 'Status' },
]

export interface EndpointGroup {
  key: string
  endpoints: Endpoint[]
}

/** What a group is called when the endpoint has no value to group by. */
const NONE: Record<Exclude<GroupBy, 'path'>, string> = { tags: 'No tag', folder: 'Top level', status: 'No status' }

const keysOf = (e: Endpoint, by: GroupBy): string[] => {
  if (by === 'path') return [e.where]
  if (by === 'tags') return e.tags.length ? e.tags : [NONE.tags]
  if (by === 'folder') return [e.folder || NONE.folder]
  return [e.status?.name || NONE.status]
}

/**
 * The endpoints in groups, in the order the list gives them within a group. An endpoint with several tags is in each of
 * them. Groups are sorted by name, with the group of endpoints that have no value last.
 */
export function groupEndpoints(list: Endpoint[], by: GroupBy): EndpointGroup[] {
  const groups = new Map<string, Endpoint[]>()
  for (const e of list) for (const k of keysOf(e, by)) groups.set(k, [...(groups.get(k) ?? []), e])
  const none = by === 'path' ? '' : NONE[by]
  return [...groups.entries()]
    .sort(([a], [b]) => (a === none ? 1 : b === none ? -1 : a.localeCompare(b)))
    .map(([key, endpoints]) => ({ key, endpoints }))
}

/** "3 methods" for a path, "3 endpoints" for the other groupings. */
export const groupCount = (n: number, by: GroupBy): string => `${n} ${by === 'path' ? (n === 1 ? 'method' : 'methods') : n === 1 ? 'endpoint' : 'endpoints'}`

/** Whether an endpoint matches words typed in the filter: any part of its verb, path, name, summary, tags or operation. */
export function endpointMatches(e: Endpoint, query: string): boolean {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean)
  const hay = [e.verb, e.where, e.path, e.name, e.summary, e.operation, ...e.tags].join(' ').toLowerCase()
  return words.every((w) => hay.includes(w))
}
