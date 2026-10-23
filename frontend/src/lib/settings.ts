// How the Settings screen arranges settings.yaml: friendly sections and labels,
// as data. A setting no rule names lands in Advanced, so a new one is never lost.

export const SECTIONS = ['Appearance', 'Requests', 'Network', 'Flows and runs', 'Replayer and server', 'Privacy', 'Advanced'] as const
export type Section = (typeof SECTIONS)[number]

// First match wins: exact names before the prefixes that would also take them.
const PLACES: [match: string, section: Section][] = [
  ['appearance.palette_file', 'Advanced'],
  ['appearance.', 'Appearance'],
  ['language', 'Appearance'],
  ['ui.', 'Appearance'],
  ['announcements_keys', 'Advanced'],
  ['announcements_dismissed', 'Advanced'],
  ['telemetry', 'Privacy'],
  ['announcements', 'Privacy'],
  ['timeout', 'Requests'],
  ['empty', 'Requests'],
  ['responses.', 'Requests'],
  ['schemas.', 'Requests'],
  ['requests.', 'Requests'],
  ['defaults.', 'Requests'],
  ['retry.', 'Requests'],
  ['http.', 'Network'],
  ['proxy.', 'Network'],
  ['tls.', 'Network'],
  ['downloads.', 'Network'],
  ['hooks.', 'Flows and runs'],
  ['run.', 'Flows and runs'],
  ['serve.', 'Replayer and server'],
]

/** The section a setting is shown in. */
export function sectionOf(name: string): Section {
  return PLACES.find(([m]) => (m.endsWith('.') || m.endsWith('_') ? name.startsWith(m) : name === m || name.startsWith(m + '_')))?.[1] ?? 'Advanced'
}

const WORDS: Record<string, string> = {
  http: 'HTTP', url: 'URL', tls: 'TLS', ca: 'CA', ui: 'interface', otel: 'OpenTelemetry', id: 'ID', addr: 'address', max: 'maximum', valkey: 'Valkey',
}
// Where the words of the name do not say it well enough.
const LABELS: Record<string, string> = {
  empty: 'Empty values in headers and query',
  'appearance.font_size': 'Font size',
  'ui.default_section': 'Request section opened first',
  'ui.suggestions_max': 'Suggestions shown while typing',
  'ui.snippet_exclude_secrets': 'Leave secrets out of code snippets',
  telemetry: 'Usage reporting',
  telemetry_endpoint: 'Usage reporting address',
  announcements: 'Show announcements',
  announcements_url: 'Announcements feed',
  announcements_keys: 'Announcement signing keys',
  announcements_dismissed: 'Dismissed announcements',
  'defaults.environment': 'Environment selected at start',
}

/** A readable label for a setting: "http.follow_redirects" is "Follow redirects". */
export function labelOf(name: string): string {
  if (LABELS[name]) return LABELS[name]
  const last = name.slice(name.lastIndexOf('.') + 1)
  const words = last.split('_').map((w) => WORDS[w] ?? w)
  const text = words.join(' ')
  return text.charAt(0).toUpperCase() + text.slice(1)
}

/** Whether a setting matches what was typed in the search box. */
export function matches(f: { name: string; description: string }, query: string): boolean {
  const q = query.trim().toLowerCase()
  return !q || [f.name, labelOf(f.name), f.description].some((t) => t.toLowerCase().includes(q))
}
