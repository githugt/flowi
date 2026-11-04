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

const isStep = (c: Command) => c.id.startsWith('step:')
const isFlowValue = (c: Command) => isStep(c) || c.id.startsWith('trace:')

/**
 * The words of the query must all appear in the title; titles that start with the query come first. A query that
 * starts with @ looks only among the steps of the open flow, and one that starts with > only among the actions
 * (the steps and traces are left out), so a short word reaches the one that is meant.
 */
export function filterCommands(commands: Command[], query: string): Command[] {
  const trimmed = query.trimStart()
  if (trimmed.startsWith('@')) return filterCommands(commands.filter(isStep), trimmed.slice(1))
  if (trimmed.startsWith('>')) return filterCommands(commands.filter((c) => !isFlowValue(c)), trimmed.slice(1))
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
  /** The steps of the open flow (empty with none open): what the step is and what it does. */
  steps: { id: string; label: string }[]
  /** The variables the open flow sets, for tracing. */
  variables: string[]
  selectStep: (id: string) => void
  traceValue: (name: string) => void
  /** Opens how environment a and environment b differ. */
  compareEnvironments: (a: string, b: string) => void
  selectEnvironment: (name: string) => void
  setTheme: (name: string) => void
}

function environmentCommands(ctx: PaletteContext): Command[] {
  const use = (name: string, title: string): Command => ({ id: 'env:' + name, title, keys: '', icon: 'variables', run: () => ctx.selectEnvironment(name) })
  const others = ctx.environments.filter((n) => n !== ctx.selected).map((n) => use(n, `Use environment ${n}`))
  if (!ctx.selected) return others
  const compare = ctx.environments
    .filter((n) => n !== ctx.selected)
    .map((n): Command => ({ id: 'compare:' + n, title: `Compare environment ${n} with ${ctx.selected}`, keys: '', icon: 'variables', run: () => ctx.compareEnvironments(ctx.selected, n) }))
  return [...others, use('', 'Use no environment'), ...compare]
}

function themeCommands(ctx: PaletteContext): Command[] {
  return ctx.themes
    .filter((t) => t !== ctx.theme)
    .map((t) => ({ id: 'theme:' + t, title: `Theme: ${t}`, keys: '', icon: 'palette', run: () => ctx.setTheme(t) }))
}

function flowCommands(ctx: PaletteContext): Command[] {
  const steps = ctx.steps.map((st): Command => ({ id: 'step:' + st.id, title: st.label && st.label !== st.id ? `Go to step: ${st.id} (${st.label})` : `Go to step: ${st.id}`, keys: '', icon: 'search', run: () => ctx.selectStep(st.id) }))
  const traces = ctx.variables.map((name): Command => ({ id: 'trace:' + name, title: `Trace value: ${name}`, keys: '', icon: 'search', run: () => ctx.traceValue(name) }))
  return [...steps, ...traces]
}

/** The commands, built each time the palette opens so the shortcuts read right for the platform. */
export function commandList(onMenu: (action: string) => void, ctx: PaletteContext): Command[] {
  const act = (id: string, title: string, keys: string, icon: string): Command => ({ id, title, keys, icon, run: () => onMenu(id) })
  const opens = screens.map((s) => act(s.menu, `Open ${s.title}`, s.menu === 'import' ? shortcut('⌘I', 'Ctrl+I') : s.menu === 'settings' ? shortcut('⌘,', 'Ctrl+,') : '', s.icon))
  return [
    act('new-request', 'New request', shortcut('⌘N', 'Ctrl+N'), 'plus'),
    act('new-flow', 'New flow', shortcut('⇧⌘N', 'Ctrl+Shift+N'), 'plus'),
    act('discover', 'Discover an API from its URL', '', 'search'),
    ...(ctx.canSend ? [act('send', 'Send the request', shortcut('⌘↵', 'Ctrl+Enter'), 'send')] : []),
    act('jump', 'Jump to a request, flow or environment', shortcut('⌘K', 'Ctrl+K'), 'search'),
    act('close-tab', 'Close tab', shortcut('⌘W', 'Ctrl+W'), 'trash'),
    act('next-tab', 'Next tab', shortcut('⇧⌘]', 'Ctrl+Shift+]'), 'down'),
    act('previous-tab', 'Previous tab', shortcut('⇧⌘[', 'Ctrl+Shift+['), 'up'),
    ...flowCommands(ctx),
    ...environmentCommands(ctx),
    ...themeCommands(ctx),
    ...opens,
  ]
}
