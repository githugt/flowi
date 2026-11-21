// The security posture of the settings: which of the choices that keep secrets and data safe are weaker than the
// built-in ones.
// Before: these settings sat in four different sections (snippets under Appearance, writes under Requests, TLS under
// Network), and nothing said whether they were weaker than the defaults.
/** The settings that make the window safer or less safe, with the value that is the safe one. */
const SAFE: { name: string; safe: string; ok: string; weak: string }[] = [
  { name: 'ui.snippet_exclude_secrets', safe: 'true', ok: 'Code snippets leave real secrets out', weak: 'Code snippets include real secrets' },
  { name: 'datasets.allow_writes', safe: 'false', ok: 'Queries cannot change data', weak: 'Queries may change data (writes allowed everywhere)' },
  { name: 'datasets.allow_outside_collection', safe: 'false', ok: 'SQL files must be inside the collection', weak: 'SQL files outside the collection may be read' },
  { name: 'secrets.host_check', safe: 'block', ok: 'A request that would send a secret to an untrusted host is stopped', weak: 'Secrets may be sent to a host you have not trusted' },
  { name: 'http.redirect_credentials', safe: 'strip', ok: 'Credentials are dropped on a redirect to another site', weak: 'Credentials follow a redirect to another site' },
]

export interface PostureLine {
  name: string
  ok: boolean
  text: string
  /** The safe value, for the Fix button. */
  safe: string
}

/** One line for each safety setting present in the list, in a fixed order: a tick when it is the safe value, a warning when not. */
export function posture(fields: { name: string; value: string }[]): PostureLine[] {
  const out: PostureLine[] = []
  for (const s of SAFE) {
    const f = fields.find((x) => x.name === s.name)
    if (!f) continue
    const ok = f.value === s.safe
    out.push({ name: s.name, ok, text: ok ? s.ok : s.weak, safe: s.safe })
  }
  return out
}

/** The lines that are weaker than the built-in choice: what "Reset to safe defaults" puts back. */
export function weakLines(lines: PostureLine[]): PostureLine[] {
  return lines.filter((l) => !l.ok)
}
