// The command palette's list: what the window's menu can do, by name, so it can be reached from the keyboard
// without knowing where it lives. Each command runs through app.onMenu, the one place a menu item is handled.
import { screens } from './screens'

export interface Command {
  id: string
  title: string
  /** The shortcut shown beside it, if it has one. */
  keys: string
  icon: string
  run: () => void
}

/** The words of the query must all appear in the title; titles that start with the query come first. */
export function filterCommands(commands: Command[], query: string): Command[] {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean)
  if (!words.length) return commands
  const matching = commands.filter((c) => words.every((w) => c.title.toLowerCase().includes(w)))
  const first = words[0]
  const starts = (c: Command) => c.title.toLowerCase().startsWith(first)
  return [...matching.filter(starts), ...matching.filter((c) => !starts(c))]
}

const isMac = () => navigator.platform.toLowerCase().includes('mac')
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
    act('close-tab', 'Close tab', shortcut('⌘W', 'Ctrl+W'), 'trash'),
    act('next-tab', 'Next tab', shortcut('⇧⌘]', 'Ctrl+Shift+]'), 'down'),
    act('previous-tab', 'Previous tab', shortcut('⇧⌘[', 'Ctrl+Shift+['), 'up'),
    ...environmentCommands(ctx),
    ...themeCommands(ctx),
    ...opens,
  ]
}
