// What the Manage environments screen shows beyond what Go sends: the items of a group menu and the groups an
// environment has after a choice.
import type { EnvGroup } from './types'
import type { MenuItem } from '../Menu.svelte'

/** The entry of an environment's groups that keeps it in the default group beside the named ones (model.DefaultGroupToken). */
export const DEFAULT_GROUP = 'default'

/** Whether an environment with these groups is in the default group: it has none, or the default entry. */
export const inDefault = (groups: string[]): boolean => groups.length === 0 || groups.includes(DEFAULT_GROUP)

/** What a chip or a menu item calls a group of an environment. */
export const groupName = (g: string): string => (g === DEFAULT_GROUP || g === '' ? 'Default' : g)

/**
 * The groups of an environment after `name` is switched on or off, sorted (the order the file keeps). Every group is its
 * own choice: the default group ("" or "default") sits beside the named ones, so choosing one never changes another.
 * An environment is always in at least one: with none left it is in the default group alone, which is how "no entry" is
 * read, so switching off the only group does nothing and "default" alone is written as no entry.
 */
export function toggledGroups(groups: string[], name: string): string[] {
  const held = groups.length === 0 ? [DEFAULT_GROUP] : groups
  const key = name === '' ? DEFAULT_GROUP : name
  const next = (held.includes(key) ? held.filter((g) => g !== key) : [...held, key]).sort()
  return next.length === 0 || (next.length === 1 && next[0] === DEFAULT_GROUP) ? [] : next
}

/**
 * The items of the menu that sets an environment's groups: the default group first, then each named group, all as ticks
 * that stay open so several can be switched in one go, then "New group…" under a rule. Each tick is its own choice.
 * `toggle` and `create` do the work, `toggle('')` being the default group; the menu is built again from what Go answers.
 */
export function groupMenuItems(groups: EnvGroup[], current: string[], toggle: (name: string) => void, create: () => void): MenuItem[] {
  return [
    ...groups.map((g) => ({ label: g.label, checked: g.name === '' ? inDefault(current) : current.includes(g.name), multi: true, count: g.count, onselect: () => toggle(g.name) })),
    { label: 'New group…', separated: groups.length > 0, onselect: create },
  ]
}
