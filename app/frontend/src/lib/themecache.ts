// The theme the window used last, kept in the browser's storage so index.html can put it on the page before
// the first paint. Before: the window drew the dark default until Go answered with the settings, so a light
// theme flashed dark at every start.
import { themeMode } from './theme'

export const THEME_KEY = 'flowi.theme'

interface Store {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
  removeItem(key: string): void
}

/** What index.html reads: the theme name and its mode, or nothing for "follow the system". */
export function themeEntry(theme: string): string {
  return theme && theme !== 'system' ? JSON.stringify({ theme, mode: themeMode(theme) }) : ''
}

/** Remembers `theme` (the named theme, or a custom palette's base); storage that throws changes nothing. */
export function rememberTheme(store: Store | undefined, theme: string): void {
  try {
    const entry = themeEntry(theme)
    if (entry) store?.setItem(THEME_KEY, entry)
    else store?.removeItem(THEME_KEY)
  } catch {
    // Storage can be blocked or full; the theme still applies once the settings arrive.
  }
}
