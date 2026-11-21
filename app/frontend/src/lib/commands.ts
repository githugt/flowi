// The command palette's list: what the window's menu can do, by name, so it can be reached from the keyboard
// without knowing where it lives. Each command runs through app.onMenu, the one place a menu item is handled.
import { screens } from './screens'
import { fold } from './fold'
import { SIDEBAR_MODES } from './sidebar'
import type { TreeNode } from './types'

export interface Command {
  id: string
  title: string
  /** The shortcut shown beside it, if it has one. */
  keys: string
  icon: string
  run: () => void
  /** Where an item lives ("Petstore › Pets"), matched by the query but shown quietly beside the title. */
  detail?: string
}

/** The words of the query must all appear in the title; titles that start with the query come first. */
export function filterCommands(commands: Command[], query: string): Command[] {
  const words = fold(query).split(/\s+/).filter(Boolean)
  if (!words.length) return commands
  const matching = commands.filter((c) => words.every((w) => fold(`${c.title} ${c.detail ?? ''}`).includes(w)))
  const first = words[0]
  const starts = (c: Command) => fold(c.title).startsWith(first)
  return [...matching.filter(starts), ...matching.filter((c) => !starts(c))]
}

export const isMac = () => navigator.platform.toLowerCase().includes('mac')
// The shortcuts of the window menu (menu.go), written for the platform.
const shortcut = (mac: string, other: string) => (isMac() ? mac : other)

/** What the window knows that decides which commands make sense now. */
export interface PaletteContext {
  environments: string[]
  /** The environment in use, "" for none. */
  selected: string
  themes: string[]
  theme: string
  /** A request is open, so sending has something to send. */
  canSend: boolean
  selectEnvironment: (name: string) => void
  setTheme: (name: string) => void
  /** Secrets are blurred on screen. Both fields are left out where the window cannot say. */
  hideSensitive?: boolean
  setHideSensitive?: (on: boolean) => void
}

function environmentCommands(ctx: PaletteContext): Command[] {
  const use = (name: string, title: string): Command => ({ id: 'env:' + name, title, keys: '', icon: 'variables', run: () => ctx.selectEnvironment(name) })
  const others = ctx.environments.filter((n) => n !== ctx.selected).map((n) => use(n, `Use environment ${n}`))
  return ctx.selected ? [...others, use('', 'Use no environment')] : others
}

function themeCommands(ctx: PaletteContext): Command[] {
  return ctx.themes
    .filter((t) => t !== ctx.theme)
    .map((t) => ({ id: 'theme:' + t, title: `Theme: ${t}`, keys: '', icon: 'palette', run: () => ctx.setTheme(t) }))
}

/** The commands, built each time the palette opens so the shortcuts read right for the platform. */
export function commandList(onMenu: (action: string) => void, ctx: PaletteContext): Command[] {
  const act = (id: string, title: string, keys: string, icon: string): Command => ({ id, title, keys, icon, run: () => onMenu(id) })
  const opens = screens.map((s) => act(s.menu, `Open ${s.title}`, s.menu === 'import' ? shortcut('⌘I', 'Ctrl+I') : s.menu === 'settings' ? shortcut('⌘,', 'Ctrl+,') : '', s.icon))
  return [
    act('new-request', 'New request', shortcut('⌘N', 'Ctrl+N'), 'plus'),
    act('new-flow', 'New flow', shortcut('⇧⌘N', 'Ctrl+Shift+N'), 'plus'),
    ...(ctx.canSend ? [act('send', 'Send the request', shortcut('⌘↵', 'Ctrl+Enter'), 'send')] : []),
    act('jump', 'Jump to a request, flow or environment', shortcut('⌘K', 'Ctrl+K'), 'search'),
    act('shortcuts', 'Keyboard shortcuts', '?', 'search'),
    act('save-all', 'Save all open documents', shortcut('⌥⌘S', 'Ctrl+Alt+S'), 'save'),
    act('toggle-sidebar', 'Fold or unfold the sidebar', shortcut('⌘B', 'Ctrl+B'), 'list'),
    ...SIDEBAR_MODES.map((m) => act('sidebar-mode:' + m.id, m.title, '', 'settings')),
    act('close-tab', 'Close tab', shortcut('⌘W', 'Ctrl+W'), 'trash'),
    act('reopen-tab', 'Reopen closed tab', shortcut('⇧⌘T', 'Ctrl+Shift+T'), 'replay'),
    act('next-tab', 'Next tab', shortcut('⇧⌘]', 'Ctrl+Shift+]'), 'down'),
    act('previous-tab', 'Previous tab', shortcut('⇧⌘[', 'Ctrl+Shift+['), 'up'),
    ...(ctx.setHideSensitive
      ? [{ id: 'hide-sensitive', title: ctx.hideSensitive ? 'Show secrets on screen again' : 'Blur secrets on screen (talk or screenshot mode)', keys: '', icon: 'auth', run: () => ctx.setHideSensitive?.(!ctx.hideSensitive) }]
      : []),
    ...environmentCommands(ctx),
    ...themeCommands(ctx),
    ...opens,
  ]
}

const ITEM_ICON: Record<string, string> = { request: 'send', flow: 'flow', environment: 'variables' }

/**
 * The requests, flows and environments of the workspace as commands that open them, so one box reaches both an
 * action and an item. `openTabs` (node ids, newest first) lead, so an empty box offers what was just in use.
 * Before: Cmd+Shift+P found commands only, and Cmd+K narrowed the tree: two boxes, neither reaching both.
 */
export function itemCommands(roots: TreeNode[], openTabs: string[], open: (n: TreeNode) => void): Command[] {
  const found: { node: TreeNode; where: string }[] = []
  const walk = (nodes: TreeNode[], where: string[]) => {
    for (const n of nodes) {
      if (ITEM_ICON[n.kind]) found.push({ node: n, where: where.join(' › ') })
      if (n.children?.length) walk(n.children, [...where, n.label])
    }
  }
  walk(roots, [])
  const rank = (id: string) => {
    const i = openTabs.indexOf(id)
    return i < 0 ? Number.MAX_SAFE_INTEGER : i
  }
  return found
    .map((f, order) => ({ ...f, order }))
    .sort((a, b) => rank(a.node.id) - rank(b.node.id) || a.order - b.order)
    .map(({ node, where }) => ({
      id: 'item:' + node.id,
      title: node.kind === 'request' && node.method ? `${node.method} ${node.label}` : node.label,
      keys: '',
      icon: ITEM_ICON[node.kind],
      detail: where,
      run: () => open(node),
    }))
}
