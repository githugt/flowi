const WORDS = new Set(['token', 'secret', 'password', 'passwd', 'pwd', 'passphrase', 'apikey', 'credential', 'credentials', 'bearer'])
// A name that ends in one of these describes the secret (where it is, how long it lives); it is not the secret.
const ABOUT = new Set(['url', 'uri', 'name', 'endpoint', 'path', 'host', 'header', 'field', 'type', 'ttl', 'expiry', 'expires', 'file', 'id'])
const VALUE = /^(sk_(live|test)_|pk_live_|ghp_|gho_|github_pat_|glpat-|xox[abprs]-|AKIA[0-9A-Z]{12}|eyJ[\w-]+\.[\w-]+\.|-----BEGIN [A-Z ]*PRIVATE KEY-----)/

function words(name: string): string[] {
  return name
    .replace(/([a-z0-9])([A-Z])/g, '$1_$2')
    .toLowerCase()
    .split(/[^a-z0-9]+/)
    .filter(Boolean)
}

// secretLike says why a variable should probably be a secret, or "" when nothing points that way.
// Before: a variable named token, password or api_key was saved as plain text in the environment file, which is
// committed, with no hint that a secret box exists.
export function secretLike(name: string, value: string): string {
  const w = words(name)
  const joined = w.join('')
  const last = w[w.length - 1]
  if (last && !ABOUT.has(last) && (w.some((x) => WORDS.has(x)) || /apikey|privatekey|accesskey|secretkey/.test(joined) || (w.includes('key') && w.some((x) => ['api', 'private', 'access', 'secret'].includes(x))))) {
    return 'The name looks like a secret.'
  }
  if (VALUE.test(value.trim())) return 'The value looks like a secret.'
  return ''
}
