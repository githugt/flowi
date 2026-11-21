import { secretLike } from './secretname'

// Which values to blur when "Blur secrets on screen" is on (ui.hide_sensitive). It is a screen filter for a talk or a
// screenshot: the value is still in the page, so nothing here is a protection against anyone with the window.
const HEADERS = new Set(['authorization', 'proxy-authorization', 'cookie', 'set-cookie', 'x-api-key', 'api-key', 'x-auth-token', 'x-csrf-token', 'x-xsrf-token'])

/** The name of a header, parameter or variable that holds a credential. */
export function sensitiveName(name: string): boolean {
  return HEADERS.has(name.trim().toLowerCase()) || secretLike(name, '') !== ''
}

/** A value to blur: named like a credential, or shaped like one (a token, a key, a JWT). */
export function sensitiveValue(name: string, value: string): boolean {
  return sensitiveName(name) || secretLike('', value) !== ''
}
