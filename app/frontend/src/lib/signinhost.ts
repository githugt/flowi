const KEY = 'flowi.signin.hosts'

/** The host of a sign-in address, or "" when it is not a web address. */
export function hostOf(url: string): string {
  try {
    const u = new URL(url)
    return u.protocol === 'https:' || u.protocol === 'http:' ? u.host : ''
  } catch {
    return ''
  }
}

/** The hosts a sign-in page was opened for before. A browser that keeps nothing gives an empty list. */
export function seenHosts(store: Pick<Storage, 'getItem'> | undefined = safeStore()): string[] {
  try {
    const raw = store?.getItem(KEY)
    const v = raw ? JSON.parse(raw) : []
    return Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : []
  } catch {
    return []
  }
}

/** Remembers a host after its page was opened. */
export function rememberHost(host: string, store: Pick<Storage, 'getItem' | 'setItem'> | undefined = safeStore()): void {
  if (!host) return
  try {
    const seen = seenHosts(store)
    if (!seen.includes(host)) store?.setItem(KEY, JSON.stringify([...seen, host].slice(-50)))
  } catch {
    /* nothing kept: the next sign-in asks again */
  }
}

function safeStore(): Storage | undefined {
  try {
    return globalThis.localStorage
  } catch {
    return undefined
  }
}

/**
 * Whether a sign-in page opens by itself: only for a host seen before. A first-use host waits for a click, so a
 * request cannot send the user to a new site without the user seeing where.
 * Before: every sign-in address opened in the browser at once.
 */
export function opensByItself(url: string, seen: readonly string[]): boolean {
  const host = hostOf(url)
  return host !== '' && seen.includes(host)
}
