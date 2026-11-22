// The window's main menu where the system has no menu of its own to show (Linux): the items, and the keyboard
// shortcuts that the native menu would have given. The actions are the ones the native menu sends (App.onMenu).
import { isLetter } from './keymatch'

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
  { label: 'Reopen closed tab', action: 'reopen-tab', shortcut: 'Ctrl+Shift+T' },
  { label: 'Next tab', action: 'next-tab', shortcut: 'Ctrl+Shift+]' },
  { label: 'Previous tab', action: 'previous-tab', shortcut: 'Ctrl+Shift+[' },
  { label: 'Home', action: 'home', separated: true },
  { label: 'Collections…', action: 'collections' },
  { label: 'Import…', action: 'import', shortcut: 'Ctrl+I' },
  { label: 'Secrets…', action: 'secrets', separated: true },
  { label: 'Manage environments…', action: 'envmanage' },
  { label: 'Compare environments…', action: 'envcompare' },
  { label: 'Schedules…', action: 'schedules' },
  { label: 'Replayer…', action: 'replayer' },
  { label: 'Checks…', action: 'checks' },
  { label: 'Authentications…', action: 'auths' },
  { label: 'Blueprints…', action: 'blueprints' },
  { label: 'Tracing…', action: 'tracing' },
  { label: 'Trash…', action: 'trash' },
  { label: 'Settings…', action: 'settings', shortcut: 'Ctrl+,', separated: true },
  { label: 'Documentation', action: 'docs', separated: true },
  { label: 'Keyboard shortcuts', action: 'shortcuts' },
  { label: 'Report a problem…', action: 'report' },
  { label: 'Welcome', action: 'welcome', separated: true },
  { label: 'Usage reporting…', action: 'telemetry' },
  { label: 'About Flowi', action: 'about' },
]

export const isLinux = (userAgent: string): boolean => /Linux/i.test(userAgent) && !/Android/i.test(userAgent)

/**
 * Whether the page itself answers the menu shortcuts: wherever the system menu is not the Mac one that carries them.
 * Before: only Linux did (isLinux), so Ctrl+N and Ctrl+W did nothing in the page on Windows when the menu bar was hidden.
 */
export const pageAnswersShortcuts = (userAgent: string): boolean => !/Macintosh|Mac OS X|iPhone|iPad|Android/i.test(userAgent)

/** F10 or Alt+M opens the main menu button, for a keyboard-only person on Linux. */
export const opensMenu = (e: { key: string; altKey: boolean; ctrlKey: boolean; metaKey: boolean; shiftKey: boolean; code: string }): boolean =>
  (e.key === 'F10' && !e.altKey && !e.ctrlKey && !e.metaKey && !e.shiftKey) || (e.altKey && !e.ctrlKey && !e.metaKey && !e.shiftKey && e.code === 'KeyM')

interface KeyLike {
  key: string
  code: string
  ctrlKey: boolean
  shiftKey: boolean
  altKey: boolean
  metaKey: boolean
}

/**
 * The rule for keys that depend on the keyboard layout, kept here so a new shortcut follows it:
 *  - letters go by isLetter: what the key types when that is a Latin letter, else the key's place (e.code) on the board;
 *  - brackets, comma and digits go by e.code, which does not move with the layout;
 *  - any press with Alt is ignored, because AltGr (Ctrl+Alt) types characters on many European layouts.
 * The tests run each rule against US, Dvorak, AZERTY, Russian and Greek presses.
 *
 * The menu action a key press stands for, or '' when it is none of the shortcuts. Letters go by isLetter, so they work
 * on a layout that does not type Latin ones.
 *
 * Before: the letters were compared by e.key, so Ctrl+N and Ctrl+W did nothing on a Russian or Greek layout, while
 * the bracket shortcuts (by e.code) did.
 */
export function actionForKey(e: KeyLike): string {
  if (!e.ctrlKey || e.altKey || e.metaKey) return ''
  const is = (letter: string) => isLetter(e, letter)
  if (e.shiftKey) {
    if (is('n')) return 'new-flow'
    if (is('p')) return 'palette'
    if (is('t')) return 'reopen-tab'
    if (e.code === 'BracketRight') return 'next-tab'
    if (e.code === 'BracketLeft') return 'previous-tab'
    return ''
  }
  if (is('n')) return 'new-request'
  if (is('k')) return 'jump'
  if (is('w')) return 'close-tab'
  if (is('i')) return 'import'
  return e.key === ',' || e.code === 'Comma' ? 'settings' : ''
}
