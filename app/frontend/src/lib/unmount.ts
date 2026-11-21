// What to tell a person before a collection is unmounted, from what the window already knows.
// Before: Unmount took one click and said nothing of the tabs it closes or the collections that depend on it.
import { plural } from './plural'
import type { TreeNode } from './types'

/** The ids of a node and everything under it. */
export function subtreeIds(root: TreeNode | undefined): Set<string> {
  const ids = new Set<string>()
  const walk = (n: TreeNode) => {
    ids.add(n.id)
    n.children?.forEach(walk)
  }
  if (root) walk(root)
  return ids
}

/** How many of the open tabs show something inside the collection. */
export function tabsIn(root: TreeNode | undefined, tabs: { node: string }[]): number {
  const ids = subtreeIds(root)
  return tabs.filter((t) => ids.has(t.node)).length
}

/** The names of the other collections that declare a dependency on this one. */
export function dependentsOf(name: string, collections: { name: string; deps: { name: string }[] }[]): string[] {
  return collections.filter((c) => c.name !== name && c.deps.some((d) => d.name === name)).map((c) => c.name)
}

/** The text of the confirmation: what stays, and what the unmount closes or breaks. */
export function unmountNote(name: string, openTabs: number, dependents: string[]): string {
  const parts = ['Your files are not touched; the collection can be mounted again.']
  if (openTabs) parts.push(`${plural(openTabs, 'tab')} open in it will be closed.`)
  if (dependents.length) parts.push(`${dependents.join(', ')} ${dependents.length === 1 ? 'depends' : 'depend'} on ${name} and will report it missing.`)
  return parts.join(' ')
}
