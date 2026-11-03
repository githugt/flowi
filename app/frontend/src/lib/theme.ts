// The themes that sit on a light background. app.css styles them through [data-mode='light'] instead of listing
// each name again in every block that needs a darker status colour or a light shadow.
const LIGHT_THEMES = ['light', 'flowi-light', 'solarized-light', 'anuppuccin-light']

/** Whether a named theme is a light or a dark one. */
export function themeMode(theme: string): 'light' | 'dark' {
  return LIGHT_THEMES.includes(theme) ? 'light' : 'dark'
}
