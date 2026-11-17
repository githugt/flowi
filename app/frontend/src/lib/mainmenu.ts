// The window's main menu where the system has no menu of its own to show (Linux): the items, and the keyboard
// shortcuts that the native menu would have given. The actions are the ones the native menu sends (App.onMenu).

export interface MainItem {
  label: string
  action: string
  /** As shown beside the label. */
  shortcut?: string
  /** A rule above this item. */
  separated?: boolean
}

export const mainItems: MainItem[] = [
  { label: 'New request', action: 'new-request', shortcut: 'Ctrl+N' },
  { label: 'New flow', action: 'new-flow', shortcut: 'Ctrl+Shift+N' },
  { label: 'Jump to…', action: 'jump', shortcut: 'Ctrl+K', separated: true },
  { label: 'Command palette…', action: 'palette', shortcut: 'Ctrl+Shift+P' },
  { label: 'Close tab', action: 'close-tab', shortcut: 'Ctrl+W' },
  { label: 'Next tab', action: 'next-tab', shortcut: 'Ctrl+Shift+]' },
  { label: 'Previous tab', action: 'previous-tab', shortcut: 'Ctrl+Shift+[' },
  { label: 'Home', action: 'home', separated: true },
  { label: 'Collections…', action: 'collections' },
  { label: 'Import…', action: 'import', shortcut: 'Ctrl+I' },
  { label: 'Secrets…', action: 'secrets', separated: true },
  { label: 'Schedules…', action: 'schedules' },
  { label: 'Replayer…', action: 'replayer' },
  { label: 'Checks…', action: 'checks' },
  { label: 'Authentications…', action: 'auths' },
  { label: 'Blueprints…', action: 'blueprints' },
  { label: 'Tracing…', action: 'tracing' },
  { label: 'Settings…', action: 'settings', shortcut: 'Ctrl+,', separated: true },
  { label: 'Welcome', action: 'welcome', separated: true },
  { label: 'Telemetry…', action: 'telemetry' },
  { label: 'About Flowi', action: 'about' },
]

export const isLinux = (userAgent: string): boolean => /Linux/i.test(userAgent) && !/Android/i.test(userAgent)

interface KeyLike {
  key: string
  code: string
  ctrlKey: boolean
  shiftKey: boolean
  altKey: boolean
  metaKey: boolean
}

/** The menu action a key press stands for, or '' when it is none of the shortcuts. */
export function actionForKey(e: KeyLike): string {
  if (!e.ctrlKey || e.altKey || e.metaKey) return ''
  const key = e.key.toLowerCase()
  if (e.shiftKey) {
    if (key === 'n') return 'new-flow'
    if (key === 'p') return 'palette'
    if (e.code === 'BracketRight') return 'next-tab'
    if (e.code === 'BracketLeft') return 'previous-tab'
    return ''
  }
  return { n: 'new-request', k: 'jump', w: 'close-tab', i: 'import', ',': 'settings' }[key] ?? ''
}
