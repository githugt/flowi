// The sidebar's three sections and its two ways of showing them. The tree from Go has a group row (Requests, Flows)
// under every collection and an Environments group beside them; the sidebar shows a Requests section and a Flows section
// instead, each listing the collections that have something for it, so no row repeats the word. Nothing is changed in
// place: a collection shown in a section is a copy whose children are the group's.
// Before: every collection repeated a Requests and a Flows row, and Environments sat among the collections.
import type { TreeNode } from './types'

/** A row as a section shows it: a collection stands for its Requests or Flows group, whose id it keeps. */
export interface RowNode extends TreeNode {
  groupId?: string
  /** Which section shows this collection: the same collection row is in more than one. */
  section?: 'requests' | 'flows'
  children?: RowNode[]
}

export type SectionId = 'requests' | 'flows' | 'environments'
export type PanelTab = 'all' | 'recent'
export type ViewMode = 'tree' | 'list'

export const SECTIONS: { id: SectionId; title: string; open: boolean }[] = [
  { id: 'requests', title: 'Requests', open: true },
  { id: 'flows', title: 'Flows', open: true },
  { id: 'environments', title: 'Environments', open: false },
]

const GROUP_OF: Record<'requests' | 'flows', string> = { requests: 'requests', flows: 'flows' }

/**
 * The rows of a section. For Requests and Flows these are the collections (the starred ones first, in the order they
 * were starred, then the others as the tree has them, with drafts last: they come and go), each standing for the
 * children of its own group. A collection with nothing for the section is left out of Flows; Requests keeps an empty
 * one, so a new collection can be seen, unless `keepEmpty` is false (a search is showing hits only).
 */
export function sectionRoots(roots: TreeNode[], section: SectionId, favourites: string[], keepEmpty = true): RowNode[] {
  if (section === 'environments') return roots.filter((r) => r.kind === 'environments').flatMap((r) => r.children ?? [])
  const picked = roots
    .filter((r) => r.kind === 'collection' || r.kind === 'missing')
    .flatMap((c): RowNode[] => {
      if (c.kind === 'missing') return section === 'requests' && keepEmpty ? [c] : []
      const group = (c.children ?? []).find((k) => k.kind === GROUP_OF[section])
      const kids = group?.children ?? []
      if (!kids.length && (section === 'flows' || !keepEmpty)) return []
      return [{ ...c, children: kids, groupId: group?.id, section }]
    })
  const rank = (c: TreeNode): number => {
    const star = favourites.indexOf(c.id)
    return star >= 0 ? star : favourites.length + Number(!!c.draft)
  }
  return picked.map((c, i) => ({ c, i })).sort((a, b) => rank(a.c) - rank(b.c) || a.i - b.i).map((x) => x.c)
}

/**
 * The row whose menu a click on this row opens. A collection in the Flows section stands for its Flows group, so its
 * menu is that group's: New flow and New folder, which makes the folder among the flows. The collection's own menu
 * makes folders among the requests.
 * Before: right-clicking a collection in Flows and choosing New folder made the folder under Requests.
 */
export const menuRowOf = (n: Pick<RowNode, 'id' | 'section' | 'groupId'>): string => (n.section === 'flows' && n.groupId ? n.groupId : n.id)

const LEAVES = ['request', 'flow', 'environment', 'broken']

/** The rows that open something, top to bottom, with the groups left out: what the list view shows. */
export function leavesOf<T extends TreeNode>(nodes: T[]): T[] {
  return nodes.flatMap((n) => (LEAVES.includes(n.kind) ? [n] : leavesOf((n.children ?? []) as T[])))
}

/** How many requests and flows (or environments) a section holds. */
export const countLeaves = (nodes: TreeNode[]): number => leavesOf(nodes).filter((n) => n.kind !== 'broken').length

/** The second line of a row in the list view: where it lives and what it is, "Test Collection / Messaging · Async". */
export function subtitle(n: Pick<TreeNode, 'where' | 'typeName'>): string {
  return [n.where, n.typeName].filter(Boolean).join(' · ')
}

/** What a rail icon that opens the sidebar floating shows: only that part of it. */
export function sectionsFor(group: '' | 'collections' | 'environments' | 'flows'): SectionId[] {
  if (group === 'collections') return ['requests']
  if (group === 'environments' || group === 'flows') return [group]
  return SECTIONS.map((s) => s.id)
}

interface Storage {
  getItem(k: string): string | null
  setItem(k: string, v: string): void
}

const VIEW_KEY = 'flowi.sidebar.view'
const TAB_KEY = 'flowi.sidebar.tab'

export function loadView(s: Storage): ViewMode {
  try {
    return s.getItem(VIEW_KEY) === 'list' ? 'list' : 'tree'
  } catch {
    return 'tree'
  }
}

export function saveView(s: Storage, v: ViewMode) {
  try {
    s.setItem(VIEW_KEY, v)
  } catch {
    // the choice just is not remembered
  }
}

export function loadTab(s: Storage): PanelTab {
  try {
    return s.getItem(TAB_KEY) === 'recent' ? 'recent' : 'all'
  } catch {
    return 'all'
  }
}

export function saveTab(s: Storage, t: PanelTab) {
  try {
    s.setItem(TAB_KEY, t)
  } catch {
    // the choice just is not remembered
  }
}
