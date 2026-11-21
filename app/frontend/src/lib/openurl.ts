// Opening a link outside the app. A collection (or a sign-in provider) supplies some of these URLs, so only web
// addresses go to the system: before this check, file:, smb: and custom handlers (ms-msdt:, vscode:) went straight
// to the OS.
import { Browser } from '@wailsio/runtime'

/** True for an absolute http or https URL: the only kind the app hands to the system. */
export function isWebUrl(url: string): boolean {
  try {
    const u = new URL(url)
    return u.protocol === 'http:' || u.protocol === 'https:'
  } catch {
    return false
  }
}

/** Opens url in the system browser when it is a web address. It returns false (and opens nothing) otherwise. */
export function openExternal(url: string): boolean {
  if (!isWebUrl(url)) return false
  void Browser.OpenURL(url)
  return true
}

/** The host a web address goes to ("api.example.com"), or '' when it is not one. */
export function linkHost(url: string): string {
  try {
    return isWebUrl(url) ? new URL(url).hostname : ''
  } catch {
    return ''
  }
}

const hostLike = /(?:[a-z0-9-]+\.)+[a-z]{2,}/gi

/**
 * The host a link's text claims when it is not the host the link goes to, else ''. A link reading "docs.example.com"
 * that opens evil.test is how a note from an imported spec or a feed hides where it leads, so the app asks first.
 * Text that names no host, or one the address is on (or a subdomain of, or the parent of), is fine.
 */
export function claimedOtherHost(text: string, url: string): string {
  const real = linkHost(url).toLowerCase()
  if (!real) return ''
  for (const m of text.match(hostLike) ?? []) {
    const claimed = m.toLowerCase().replace(/^www\./, '')
    const same = real === claimed || real.endsWith('.' + claimed) || claimed.endsWith('.' + real)
    if (!same) return claimed
  }
  return ''
}
